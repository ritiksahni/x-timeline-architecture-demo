package timeline

import (
	"context"
	"fmt"
	"time"

	"github.com/ritik/twitter-fan-out/internal/cache"
	"github.com/ritik/twitter-fan-out/internal/models"
	"github.com/ritik/twitter-fan-out/internal/repository"
)

// HybridStrategy implements Twitter's hybrid approach
// - Regular users (< threshold followers): fan-out on write
// - Celebrities (>= threshold followers): fan-out on read
type HybridStrategy struct {
	tweetRepo          *repository.TweetRepository
	followRepo         *repository.FollowRepository
	userRepo           *repository.UserRepository
	cache              *cache.TimelineCache
	celebrityThreshold int
}

// NewHybridStrategy creates a new HybridStrategy
func NewHybridStrategy(
	tweetRepo *repository.TweetRepository,
	followRepo *repository.FollowRepository,
	userRepo *repository.UserRepository,
	cache *cache.TimelineCache,
	celebrityThreshold int,
) *HybridStrategy {
	return &HybridStrategy{
		tweetRepo:          tweetRepo,
		followRepo:         followRepo,
		userRepo:           userRepo,
		cache:              cache,
		celebrityThreshold: celebrityThreshold,
	}
}

// Name returns the strategy name
func (s *HybridStrategy) Name() string {
	return "hybrid"
}

// SetCelebrityThreshold updates the celebrity threshold
func (s *HybridStrategy) SetCelebrityThreshold(threshold int) {
	s.celebrityThreshold = threshold
}

// GetCelebrityThreshold returns the current celebrity threshold
func (s *HybridStrategy) GetCelebrityThreshold() int {
	return s.celebrityThreshold
}

// PostTweet creates a tweet with hybrid fan-out logic
func (s *HybridStrategy) PostTweet(ctx context.Context, userID int64, content string) (*models.Tweet, *OperationMetrics, error) {
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

	// 2. Get the author's info to check if they're a celebrity
	author, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		metrics.Error = err
		metrics.EndTime = time.Now()
		return tweet, metrics, fmt.Errorf("failed to get author: %w", err)
	}

	tweet.Username = author.Username

	// 3. Cache the tweet data
	s.cache.CacheTweet(ctx, tweet)

	// 4. Decide fan-out strategy based on follower count
	isCelebrity := author.IsCelebrity(s.celebrityThreshold)

	if isCelebrity {
		// Celebrity: store in celebrity tweets cache, don't fan out
		if err := s.cache.AddCelebrityTweet(ctx, userID, tweet); err != nil {
			fmt.Printf("Warning: failed to add celebrity tweet: %v\n", err)
		}
		metrics.FanOutCount = 0
	} else {
		// Regular user: fan out to all followers
		followers, err := s.followRepo.GetFollowers(ctx, userID)
		if err != nil {
			metrics.Error = err
			metrics.EndTime = time.Now()
			return tweet, metrics, fmt.Errorf("failed to get followers: %w", err)
		}

		metrics.FanOutCount = len(followers)

		if len(followers) > 0 {
			fanOutStart := time.Now()
			if err := s.cache.AddToTimelineBatch(ctx, followers, tweet); err != nil {
				fmt.Printf("Warning: failed to fan out to some timelines: %v\n", err)
			}
			metrics.FanOutDuration = time.Since(fanOutStart)
		}
	}

	// 5. Add to author's own timeline
	if err := s.cache.AddToTimeline(ctx, userID, tweet); err != nil {
		fmt.Printf("Warning: failed to add to author's timeline: %v\n", err)
	}

	metrics.EndTime = time.Now()
	metrics.Success = true

	return tweet, metrics, nil
}

// GetTimeline retrieves a user's timeline using hybrid approach
func (s *HybridStrategy) GetTimeline(ctx context.Context, userID int64, limit, offset int) ([]*models.Tweet, *OperationMetrics, error) {
	metrics := &OperationMetrics{
		Strategy:  s.Name(),
		Operation: "get_timeline",
		StartTime: time.Now(),
	}

	// 1. Get pre-computed timeline from cache (tweets from non-celebrities)
	cachedTweetIDs, err := s.cache.GetTimeline(ctx, userID, limit*2, 0) // Get more to account for merging
	if err != nil {
		fmt.Printf("Warning: failed to get cached timeline: %v\n", err)
		cachedTweetIDs = []int64{}
	}

	var cachedTweets []*models.Tweet
	if len(cachedTweetIDs) > 0 {
		metrics.CacheHit = true
		
		// Get tweet data from cache or DB
		cachedTweets, _, err = s.cache.GetCachedTweets(ctx, cachedTweetIDs)
		if err != nil || len(cachedTweets) < len(cachedTweetIDs) {
			// Fetch missing from DB
			cachedTweets, err = s.tweetRepo.GetByIDs(ctx, cachedTweetIDs)
			if err != nil {
				cachedTweets = []*models.Tweet{}
			}
		}
	}

	// 2. Get celebrities this user follows
	celebrities, err := s.followRepo.GetFollowingCelebrities(ctx, userID, s.celebrityThreshold)
	if err != nil {
		fmt.Printf("Warning: failed to get following celebrities: %v\n", err)
		celebrities = []*models.User{}
	}

	// 3. Fetch recent tweets from celebrities (fan-out on read for celebrities)
	var celebrityTweets []*models.Tweet
	if len(celebrities) > 0 {
		celebrityIDs := make([]int64, len(celebrities))
		for i, c := range celebrities {
			celebrityIDs[i] = c.ID
		}

		// Try to get from celebrity cache first
		celebrityTweetIDs, err := s.cache.GetCelebrityTweetsBatch(ctx, celebrityIDs, 20)
		if err == nil && len(celebrityTweetIDs) > 0 {
			celebrityTweets, _, _ = s.cache.GetCachedTweets(ctx, celebrityTweetIDs)
		}

		// If cache miss or incomplete, fetch from DB
		if len(celebrityTweets) < len(celebrities)*5 { // Expect at least some tweets per celebrity
			dbTweets, err := s.tweetRepo.GetRecentByUserIDs(ctx, celebrityIDs, 10, limit)
			if err == nil {
				celebrityTweets = append(celebrityTweets, dbTweets...)
				celebrityTweets = deduplicateTweets(celebrityTweets)
			}
		}

		metrics.FanOutCount = len(celebrities) // Number of celebrities merged at read time
	}

	// 4. Merge cached timeline with celebrity tweets
	allTweets := mergeTweets(cachedTweets, celebrityTweets, limit*2)

	// 5. Deduplicate (in case of any overlap)
	allTweets = deduplicateTweets(allTweets)

	// 6. Sort by time
	sortTweetsByTime(allTweets)

	// 7. Apply offset and limit
	if offset > 0 {
		if offset >= len(allTweets) {
			allTweets = []*models.Tweet{}
		} else {
			allTweets = allTweets[offset:]
		}
	}
	if len(allTweets) > limit {
		allTweets = allTweets[:limit]
	}

	// 8. Cache any tweets we fetched from DB
	s.cache.CacheTweetsBatch(ctx, allTweets)

	metrics.EndTime = time.Now()
	metrics.Success = true

	return allTweets, metrics, nil
}

// RebuildTimeline rebuilds a user's timeline cache (for non-celebrity tweets only)
func (s *HybridStrategy) RebuildTimeline(ctx context.Context, userID int64, limit int) error {
	// Get non-celebrity users this person follows
	nonCelebrityIDs, err := s.followRepo.GetFollowingNonCelebrities(ctx, userID, s.celebrityThreshold)
	if err != nil {
		return fmt.Errorf("failed to get following non-celebrities: %w", err)
	}

	// Include user's own tweets if they're not a celebrity
	user, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && !user.IsCelebrity(s.celebrityThreshold) {
		nonCelebrityIDs = append(nonCelebrityIDs, userID)
	}

	if len(nonCelebrityIDs) == 0 {
		return nil
	}

	// Get recent tweets from non-celebrities
	tweets, err := s.tweetRepo.GetByUserIDs(ctx, nonCelebrityIDs, limit)
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

// DeleteTweet removes a tweet with appropriate cache invalidation
func (s *HybridStrategy) DeleteTweet(ctx context.Context, tweetID int64, userID int64) error {
	// Get author info
	author, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	if author.IsCelebrity(s.celebrityThreshold) {
		// Celebrity: just delete from DB and celebrity cache
		// Followers will naturally not see it on next read
	} else {
		// Regular user: need to remove from all followers' caches
		followers, err := s.followRepo.GetFollowers(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get followers: %w", err)
		}

		if len(followers) > 0 {
			if err := s.cache.RemoveFromTimelineBatch(ctx, followers, tweetID); err != nil {
				fmt.Printf("Warning: failed to remove from some timelines: %v\n", err)
			}
		}
	}

	// Remove from author's timeline
	s.cache.RemoveFromTimeline(ctx, userID, tweetID)

	// Delete from database
	return s.tweetRepo.Delete(ctx, tweetID)
}
