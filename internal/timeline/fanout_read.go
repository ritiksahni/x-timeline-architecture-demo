package timeline

import (
	"context"
	"fmt"
	"time"

	"github.com/ritik/twitter-fan-out/internal/cache"
	"github.com/ritik/twitter-fan-out/internal/models"
	"github.com/ritik/twitter-fan-out/internal/repository"
)

// FanOutReadStrategy implements the fan-out-on-read approach
// Tweets are stored once, and timelines are computed at read time
type FanOutReadStrategy struct {
	tweetRepo  *repository.TweetRepository
	followRepo *repository.FollowRepository
	userRepo   *repository.UserRepository
	cache      *cache.TimelineCache
}

// NewFanOutReadStrategy creates a new FanOutReadStrategy
func NewFanOutReadStrategy(
	tweetRepo *repository.TweetRepository,
	followRepo *repository.FollowRepository,
	userRepo *repository.UserRepository,
	cache *cache.TimelineCache,
) *FanOutReadStrategy {
	return &FanOutReadStrategy{
		tweetRepo:  tweetRepo,
		followRepo: followRepo,
		userRepo:   userRepo,
		cache:      cache,
	}
}

// Name returns the strategy name
func (s *FanOutReadStrategy) Name() string {
	return "fanout_read"
}

// PostTweet creates a tweet - simple O(1) operation
func (s *FanOutReadStrategy) PostTweet(ctx context.Context, userID int64, content string) (*models.Tweet, *OperationMetrics, error) {
	metrics := &OperationMetrics{
		Strategy:  s.Name(),
		Operation: "post_tweet",
		StartTime: time.Now(),
	}

	// Simply create the tweet in PostgreSQL - no fan-out needed
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

	// Optionally cache the tweet for faster retrieval
	s.cache.CacheTweet(ctx, tweet)

	metrics.EndTime = time.Now()
	metrics.Success = true
	metrics.FanOutCount = 0 // No fan-out in this strategy

	return tweet, metrics, nil
}

// GetTimeline computes the timeline at read time by fetching from all followed users
func (s *FanOutReadStrategy) GetTimeline(ctx context.Context, userID int64, limit, offset int) ([]*models.Tweet, *OperationMetrics, error) {
	metrics := &OperationMetrics{
		Strategy:  s.Name(),
		Operation: "get_timeline",
		StartTime: time.Now(),
	}

	// 1. Get list of users this person follows
	following, err := s.followRepo.GetFollowing(ctx, userID)
	if err != nil {
		metrics.Error = err
		metrics.EndTime = time.Now()
		return nil, metrics, fmt.Errorf("failed to get following: %w", err)
	}

	// Include user's own tweets in their timeline
	following = append(following, userID)

	metrics.FanOutCount = len(following) // In read strategy, this represents the merge count

	if len(following) == 0 {
		metrics.EndTime = time.Now()
		metrics.Success = true
		return []*models.Tweet{}, metrics, nil
	}

	// 2. Fetch recent tweets from all followed users
	// Using a lateral join query for efficiency
	tweets, err := s.tweetRepo.GetRecentByUserIDs(ctx, following, 10, limit+offset)
	if err != nil {
		// Fall back to simpler query
		tweets, err = s.tweetRepo.GetByUserIDs(ctx, following, limit+offset)
		if err != nil {
			metrics.Error = err
			metrics.EndTime = time.Now()
			return nil, metrics, fmt.Errorf("failed to get tweets: %w", err)
		}
	}

	// 3. Sort by time (should already be sorted from DB, but ensure)
	sortTweetsByTime(tweets)

	// 4. Apply offset and limit
	if offset > 0 {
		if offset >= len(tweets) {
			tweets = []*models.Tweet{}
		} else {
			tweets = tweets[offset:]
		}
	}
	if len(tweets) > limit {
		tweets = tweets[:limit]
	}

	// 5. Cache tweets for potential future use
	s.cache.CacheTweetsBatch(ctx, tweets)

	metrics.EndTime = time.Now()
	metrics.Success = true
	metrics.CacheHit = false // Fan-out-read doesn't use timeline cache

	return tweets, metrics, nil
}

// DeleteTweet simply deletes the tweet - no cache invalidation needed
func (s *FanOutReadStrategy) DeleteTweet(ctx context.Context, tweetID int64, userID int64) error {
	return s.tweetRepo.Delete(ctx, tweetID)
}
