package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ritik/twitter-fan-out/internal/models"
)

const (
	// Key prefixes
	timelineKeyPrefix       = "timeline:"
	tweetCacheKeyPrefix     = "tweet:"
	celebrityTweetsPrefix   = "celebrity:tweets:"
	
	// TTL settings
	tweetCacheTTL    = 24 * time.Hour
	timelineCacheTTL = 7 * 24 * time.Hour
)

// TimelineCache handles timeline caching operations
type TimelineCache struct {
	client        *redis.Client
	maxTimelineSize int
}

// NewTimelineCache creates a new TimelineCache
func NewTimelineCache(client *redis.Client, maxSize int) *TimelineCache {
	return &TimelineCache{
		client:        client,
		maxTimelineSize: maxSize,
	}
}

// timelineKey returns the Redis key for a user's timeline
func timelineKey(userID int64) string {
	return fmt.Sprintf("%s%d", timelineKeyPrefix, userID)
}

// tweetCacheKey returns the Redis key for a cached tweet
func tweetCacheKey(tweetID int64) string {
	return fmt.Sprintf("%s%d", tweetCacheKeyPrefix, tweetID)
}

// celebrityTweetsKey returns the Redis key for a celebrity's tweets
func celebrityTweetsKey(userID int64) string {
	return fmt.Sprintf("%s%d", celebrityTweetsPrefix, userID)
}

// AddToTimeline adds a tweet to a user's timeline cache
func (tc *TimelineCache) AddToTimeline(ctx context.Context, userID int64, tweet *models.Tweet) error {
	key := timelineKey(userID)
	score := float64(tweet.CreatedAt.UnixNano())
	
	// Add tweet ID to sorted set
	err := tc.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: tweet.ID,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add to timeline: %w", err)
	}

	// Trim to max size (keep most recent)
	err = tc.client.ZRemRangeByRank(ctx, key, 0, int64(-tc.maxTimelineSize-1)).Err()
	if err != nil {
		return fmt.Errorf("failed to trim timeline: %w", err)
	}

	// Set TTL
	tc.client.Expire(ctx, key, timelineCacheTTL)

	return nil
}

// AddToTimelineBatch adds a tweet to multiple users' timelines (fan-out)
func (tc *TimelineCache) AddToTimelineBatch(ctx context.Context, userIDs []int64, tweet *models.Tweet) error {
	if len(userIDs) == 0 {
		return nil
	}

	pipe := tc.client.Pipeline()
	score := float64(tweet.CreatedAt.UnixNano())

	for _, userID := range userIDs {
		key := timelineKey(userID)
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  score,
			Member: tweet.ID,
		})
		pipe.ZRemRangeByRank(ctx, key, 0, int64(-tc.maxTimelineSize-1))
		pipe.Expire(ctx, key, timelineCacheTTL)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to batch add to timelines: %w", err)
	}

	return nil
}

// GetTimeline retrieves tweet IDs from a user's timeline cache
func (tc *TimelineCache) GetTimeline(ctx context.Context, userID int64, limit, offset int) ([]int64, error) {
	key := timelineKey(userID)
	
	// Get tweet IDs in reverse chronological order
	results, err := tc.client.ZRevRange(ctx, key, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}

	tweetIDs := make([]int64, 0, len(results))
	for _, r := range results {
		id, err := strconv.ParseInt(r, 10, 64)
		if err != nil {
			continue
		}
		tweetIDs = append(tweetIDs, id)
	}

	return tweetIDs, nil
}

// GetTimelineSize returns the number of tweets in a user's timeline cache
func (tc *TimelineCache) GetTimelineSize(ctx context.Context, userID int64) (int64, error) {
	key := timelineKey(userID)
	return tc.client.ZCard(ctx, key).Result()
}

// RemoveFromTimeline removes a tweet from a user's timeline
func (tc *TimelineCache) RemoveFromTimeline(ctx context.Context, userID int64, tweetID int64) error {
	key := timelineKey(userID)
	return tc.client.ZRem(ctx, key, tweetID).Err()
}

// RemoveFromTimelineBatch removes a tweet from multiple users' timelines
func (tc *TimelineCache) RemoveFromTimelineBatch(ctx context.Context, userIDs []int64, tweetID int64) error {
	if len(userIDs) == 0 {
		return nil
	}

	pipe := tc.client.Pipeline()
	for _, userID := range userIDs {
		key := timelineKey(userID)
		pipe.ZRem(ctx, key, tweetID)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ClearTimeline clears a user's timeline cache
func (tc *TimelineCache) ClearTimeline(ctx context.Context, userID int64) error {
	key := timelineKey(userID)
	return tc.client.Del(ctx, key).Err()
}

// CacheTweet caches a tweet's data
func (tc *TimelineCache) CacheTweet(ctx context.Context, tweet *models.Tweet) error {
	key := tweetCacheKey(tweet.ID)
	data, err := json.Marshal(tweet)
	if err != nil {
		return fmt.Errorf("failed to marshal tweet: %w", err)
	}
	return tc.client.Set(ctx, key, data, tweetCacheTTL).Err()
}

// GetCachedTweet retrieves a cached tweet
func (tc *TimelineCache) GetCachedTweet(ctx context.Context, tweetID int64) (*models.Tweet, error) {
	key := tweetCacheKey(tweetID)
	data, err := tc.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cached tweet: %w", err)
	}

	tweet := &models.Tweet{}
	if err := json.Unmarshal(data, tweet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tweet: %w", err)
	}
	return tweet, nil
}

// GetCachedTweets retrieves multiple cached tweets
func (tc *TimelineCache) GetCachedTweets(ctx context.Context, tweetIDs []int64) ([]*models.Tweet, []int64, error) {
	if len(tweetIDs) == 0 {
		return []*models.Tweet{}, []int64{}, nil
	}

	keys := make([]string, len(tweetIDs))
	for i, id := range tweetIDs {
		keys[i] = tweetCacheKey(id)
	}

	results, err := tc.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cached tweets: %w", err)
	}

	tweets := make([]*models.Tweet, 0, len(tweetIDs))
	missingIDs := make([]int64, 0)

	for i, result := range results {
		if result == nil {
			missingIDs = append(missingIDs, tweetIDs[i])
			continue
		}

		data, ok := result.(string)
		if !ok {
			missingIDs = append(missingIDs, tweetIDs[i])
			continue
		}

		tweet := &models.Tweet{}
		if err := json.Unmarshal([]byte(data), tweet); err != nil {
			missingIDs = append(missingIDs, tweetIDs[i])
			continue
		}
		tweets = append(tweets, tweet)
	}

	return tweets, missingIDs, nil
}

// CacheTweetsBatch caches multiple tweets
func (tc *TimelineCache) CacheTweetsBatch(ctx context.Context, tweets []*models.Tweet) error {
	if len(tweets) == 0 {
		return nil
	}

	pipe := tc.client.Pipeline()
	for _, tweet := range tweets {
		key := tweetCacheKey(tweet.ID)
		data, err := json.Marshal(tweet)
		if err != nil {
			continue
		}
		pipe.Set(ctx, key, data, tweetCacheTTL)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// AddCelebrityTweet adds a tweet to a celebrity's tweet cache (for hybrid approach)
func (tc *TimelineCache) AddCelebrityTweet(ctx context.Context, userID int64, tweet *models.Tweet) error {
	key := celebrityTweetsKey(userID)
	score := float64(tweet.CreatedAt.UnixNano())
	
	err := tc.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: tweet.ID,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to add celebrity tweet: %w", err)
	}

	// Keep only recent tweets
	tc.client.ZRemRangeByRank(ctx, key, 0, -101) // Keep last 100
	tc.client.Expire(ctx, key, timelineCacheTTL)

	return nil
}

// GetCelebrityTweets retrieves recent tweet IDs from a celebrity
func (tc *TimelineCache) GetCelebrityTweets(ctx context.Context, userID int64, limit int) ([]int64, error) {
	key := celebrityTweetsKey(userID)
	
	results, err := tc.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get celebrity tweets: %w", err)
	}

	tweetIDs := make([]int64, 0, len(results))
	for _, r := range results {
		id, err := strconv.ParseInt(r, 10, 64)
		if err != nil {
			continue
		}
		tweetIDs = append(tweetIDs, id)
	}

	return tweetIDs, nil
}

// GetCelebrityTweetsBatch retrieves recent tweets from multiple celebrities
func (tc *TimelineCache) GetCelebrityTweetsBatch(ctx context.Context, userIDs []int64, limitPerUser int) ([]int64, error) {
	if len(userIDs) == 0 {
		return []int64{}, nil
	}

	pipe := tc.client.Pipeline()
	cmds := make([]*redis.StringSliceCmd, len(userIDs))
	
	for i, userID := range userIDs {
		key := celebrityTweetsKey(userID)
		cmds[i] = pipe.ZRevRange(ctx, key, 0, int64(limitPerUser-1))
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get celebrity tweets batch: %w", err)
	}

	allTweetIDs := make([]int64, 0)
	for _, cmd := range cmds {
		results, err := cmd.Result()
		if err != nil {
			continue
		}
		for _, r := range results {
			id, err := strconv.ParseInt(r, 10, 64)
			if err != nil {
				continue
			}
			allTweetIDs = append(allTweetIDs, id)
		}
	}

	return allTweetIDs, nil
}

// TimelineExists checks if a user has a cached timeline
func (tc *TimelineCache) TimelineExists(ctx context.Context, userID int64) (bool, error) {
	key := timelineKey(userID)
	exists, err := tc.client.Exists(ctx, key).Result()
	return exists > 0, err
}
