package timeline

import (
	"context"
	"fmt"
	"time"

	"github.com/ritik/twitter-fan-out/internal/cache"
	"github.com/ritik/twitter-fan-out/internal/models"
	"github.com/ritik/twitter-fan-out/internal/repository"
)

// FanOutWriteStrategy implements the fan-out-on-write approach
// When a user posts a tweet, it's immediately pushed to all followers' timeline caches
type FanOutWriteStrategy struct {
	tweetRepo  *repository.TweetRepository
	followRepo *repository.FollowRepository
	userRepo   *repository.UserRepository
	cache      *cache.TimelineCache
}

// NewFanOutWriteStrategy creates a new FanOutWriteStrategy
func NewFanOutWriteStrategy(
	tweetRepo *repository.TweetRepository,
	followRepo *repository.FollowRepository,
	userRepo *repository.UserRepository,
	cache *cache.TimelineCache,
) *FanOutWriteStrategy {
	return &FanOutWriteStrategy{
		tweetRepo:  tweetRepo,
		followRepo: followRepo,
		userRepo:   userRepo,
		cache:      cache,
	}
}

// Name returns the strategy name
func (s *FanOutWriteStrategy) Name() string {
	return "fanout_write"
}

// PostTweet creates a tweet and fans out to all followers' caches
func (s *FanOutWriteStrategy) PostTweet(ctx context.Context, userID int64, content string) (*models.Tweet, *OperationMetrics, error) {
	metrics := &OperationMetrics{
		Strategy:  s.Name(),
		Operation: "post_tweet",
		StartTime: time.Now(),
	}

	// 1. Create the tweet in PostgreSQL
	tweet, err := s.tweetRepo.Create(ctx, userID, content)
	if err != nil {
		metrics.Error = err
		metrics.EndTime = time.Now()
		return nil, metrics, fmt.Errorf("failed to create tweet: %w", err)
	}

	// Get username for the tweet
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil {
		tweet.Username = user.Username
	}

	// 2. Cache the tweet data
	if err := s.cache.CacheTweet(ctx, tweet); err != nil {
		// Log but don't fail - tweet is already persisted
		fmt.Printf("Warning: failed to cache tweet: %v\n", err)
	}

	// 3. Get all followers
	followers, err := s.followRepo.GetFollowers(ctx, userID)
	if err != nil {
		metrics.Error = err
		metrics.EndTime = time.Now()
		return tweet, metrics, fmt.Errorf("failed to get followers: %w", err)
	}

	metrics.FanOutCount = len(followers)

	// 4. Fan out to all followers' timelines
	if len(followers) > 0 {
		fanOutStart := time.Now()
		if err := s.cache.AddToTimelineBatch(ctx, followers, tweet); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to fan out to some timelines: %v\n", err)
		}
		metrics.FanOutDuration = time.Since(fanOutStart)
	}

	// 5. Also add to the author's own timeline
	if err := s.cache.AddToTimeline(ctx, userID, tweet); err != nil {
		fmt.Printf("Warning: failed to add to author's timeline: %v\n", err)
	}

	metrics.EndTime = time.Now()
	metrics.Success = true

	return tweet, metrics, nil
}

// GetTimeline retrieves a user's timeline from cache
func (s *FanOutWriteStrategy) GetTimeline(ctx context.Context, userID int64, limit, offset int) ([]*models.Tweet, *OperationMetrics, error) {
	metrics := &OperationMetrics{
		Strategy:  s.Name(),
		Operation: "get_timeline",
		StartTime: time.Now(),
	}

	// 1. Get tweet IDs from cache
	tweetIDs, err := s.cache.GetTimeline(ctx, userID, limit, offset)
	if err != nil {
		metrics.Error = err
		metrics.EndTime = time.Now()
		return nil, metrics, fmt.Errorf("failed to get timeline from cache: %w", err)
	}

	if len(tweetIDs) == 0 {
		// Timeline is empty or not cached - could rebuild from DB
		metrics.CacheHit = false
		metrics.EndTime = time.Now()
		metrics.Success = true
		return []*models.Tweet{}, metrics, nil
	}

	metrics.CacheHit = true

	// 2. Try to get tweets from cache first
	tweets, missingIDs, err := s.cache.GetCachedTweets(ctx, tweetIDs)
	if err != nil {
		// Fall back to DB
		missingIDs = tweetIDs
		tweets = []*models.Tweet{}
	}

	// 3. Fetch missing tweets from DB
	if len(missingIDs) > 0 {
		dbTweets, err := s.tweetRepo.GetByIDs(ctx, missingIDs)
		if err != nil {
			metrics.Error = err
			metrics.EndTime = time.Now()
			return tweets, metrics, fmt.Errorf("failed to get tweets from DB: %w", err)
		}
		tweets = append(tweets, dbTweets...)

		// Cache the fetched tweets
		s.cache.CacheTweetsBatch(ctx, dbTweets)
	}

	// 4. Sort tweets by created_at (most recent first)
	sortTweetsByTime(tweets)

	metrics.EndTime = time.Now()
	metrics.Success = true

	return tweets, metrics, nil
}

// RebuildTimeline rebuilds a user's timeline cache from scratch
func (s *FanOutWriteStrategy) RebuildTimeline(ctx context.Context, userID int64, limit int) error {
	// Get users this person follows
	following, err := s.followRepo.GetFollowing(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get following: %w", err)
	}

	// Include user's own tweets
	following = append(following, userID)

	// Get recent tweets from all followed users
	tweets, err := s.tweetRepo.GetByUserIDs(ctx, following, limit)
	if err != nil {
		return fmt.Errorf("failed to get tweets: %w", err)
	}

	// Clear existing timeline
	s.cache.ClearTimeline(ctx, userID)

	// Add tweets to timeline
	for _, tweet := range tweets {
		if err := s.cache.AddToTimeline(ctx, userID, tweet); err != nil {
			return fmt.Errorf("failed to add tweet to timeline: %w", err)
		}
	}

	// Cache tweet data
	s.cache.CacheTweetsBatch(ctx, tweets)

	return nil
}

// DeleteTweet removes a tweet and updates all followers' caches
func (s *FanOutWriteStrategy) DeleteTweet(ctx context.Context, tweetID int64, userID int64) error {
	// Get followers to update their caches
	followers, err := s.followRepo.GetFollowers(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	// Remove from all followers' timelines
	if len(followers) > 0 {
		if err := s.cache.RemoveFromTimelineBatch(ctx, followers, tweetID); err != nil {
			fmt.Printf("Warning: failed to remove from some timelines: %v\n", err)
		}
	}

	// Remove from author's timeline
	s.cache.RemoveFromTimeline(ctx, userID, tweetID)

	// Delete from database
	return s.tweetRepo.Delete(ctx, tweetID)
}
