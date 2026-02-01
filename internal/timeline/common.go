package timeline

import (
	"context"
	"sort"
	"time"

	"github.com/ritik/twitter-fan-out/internal/models"
)

// Strategy defines the interface for timeline strategies
type Strategy interface {
	Name() string
	PostTweet(ctx context.Context, userID int64, content string) (*models.Tweet, *OperationMetrics, error)
	GetTimeline(ctx context.Context, userID int64, limit, offset int) ([]*models.Tweet, *OperationMetrics, error)
}

// OperationMetrics holds metrics for a single operation
type OperationMetrics struct {
	Strategy       string
	Operation      string
	StartTime      time.Time
	EndTime        time.Time
	FanOutCount    int           // Number of users fanned out to
	FanOutDuration time.Duration // Time spent on fan-out
	CacheHit       bool
	Success        bool
	Error          error
}

// Duration returns the total operation duration
func (m *OperationMetrics) Duration() time.Duration {
	return m.EndTime.Sub(m.StartTime)
}

// sortTweetsByTime sorts tweets by created_at in descending order (most recent first)
func sortTweetsByTime(tweets []*models.Tweet) {
	sort.Slice(tweets, func(i, j int) bool {
		return tweets[i].CreatedAt.After(tweets[j].CreatedAt)
	})
}

// mergeTweets merges two sorted tweet slices and returns top N
func mergeTweets(a, b []*models.Tweet, limit int) []*models.Tweet {
	result := make([]*models.Tweet, 0, len(a)+len(b))
	result = append(result, a...)
	result = append(result, b...)
	
	sortTweetsByTime(result)
	
	if len(result) > limit {
		return result[:limit]
	}
	return result
}

// deduplicateTweets removes duplicate tweets by ID
func deduplicateTweets(tweets []*models.Tweet) []*models.Tweet {
	seen := make(map[int64]bool)
	result := make([]*models.Tweet, 0, len(tweets))
	
	for _, tweet := range tweets {
		if !seen[tweet.ID] {
			seen[tweet.ID] = true
			result = append(result, tweet)
		}
	}
	
	return result
}

// StrategyType represents the type of timeline strategy
type StrategyType string

const (
	StrategyFanOutWrite StrategyType = "fanout_write"
	StrategyFanOutRead  StrategyType = "fanout_read"
	StrategyHybrid      StrategyType = "hybrid"
)

// ValidStrategies returns all valid strategy types
func ValidStrategies() []StrategyType {
	return []StrategyType{
		StrategyFanOutWrite,
		StrategyFanOutRead,
		StrategyHybrid,
	}
}

// IsValidStrategy checks if a strategy type is valid
func IsValidStrategy(s string) bool {
	for _, valid := range ValidStrategies() {
		if string(valid) == s {
			return true
		}
	}
	return false
}
