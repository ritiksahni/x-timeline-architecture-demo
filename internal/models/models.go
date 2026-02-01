package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID            int64     `json:"id" db:"id"`
	Username      string    `json:"username" db:"username"`
	FollowerCount int       `json:"follower_count" db:"follower_count"`
	FollowingCount int      `json:"following_count" db:"following_count"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// IsCelebrity returns true if the user has more followers than the threshold
func (u *User) IsCelebrity(threshold int) bool {
	return u.FollowerCount >= threshold
}

// Tweet represents a tweet/post
type Tweet struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	
	// Joined fields (not stored in DB)
	Username  string    `json:"username,omitempty" db:"username"`
}

// TweetWithAuthor includes author information
type TweetWithAuthor struct {
	Tweet
	Author *User `json:"author,omitempty"`
}

// Follow represents a follow relationship
type Follow struct {
	FollowerID  int64     `json:"follower_id" db:"follower_id"`
	FolloweeID  int64     `json:"followee_id" db:"followee_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Timeline represents a user's timeline
type Timeline struct {
	UserID int64    `json:"user_id"`
	Tweets []*Tweet `json:"tweets"`
}

// TimelineRequest represents a request to fetch a timeline
type TimelineRequest struct {
	UserID   int64  `json:"user_id"`
	Strategy string `json:"strategy"` // "fanout_write", "fanout_read", "hybrid"
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// PostTweetRequest represents a request to post a tweet
type PostTweetRequest struct {
	UserID   int64  `json:"user_id"`
	Content  string `json:"content"`
	Strategy string `json:"strategy"` // "fanout_write", "fanout_read", "hybrid"
}

// BenchmarkResult holds the results of a benchmark run
type BenchmarkResult struct {
	Strategy        string        `json:"strategy"`
	TotalTweets     int           `json:"total_tweets"`
	TotalReads      int           `json:"total_reads"`
	WriteLatencyP50 time.Duration `json:"write_latency_p50"`
	WriteLatencyP95 time.Duration `json:"write_latency_p95"`
	WriteLatencyP99 time.Duration `json:"write_latency_p99"`
	WriteLatencyAvg time.Duration `json:"write_latency_avg"`
	ReadLatencyP50  time.Duration `json:"read_latency_p50"`
	ReadLatencyP95  time.Duration `json:"read_latency_p95"`
	ReadLatencyP99  time.Duration `json:"read_latency_p99"`
	ReadLatencyAvg  time.Duration `json:"read_latency_avg"`
	WriteThroughput float64       `json:"write_throughput"` // tweets/sec
	ReadThroughput  float64       `json:"read_throughput"`  // reads/sec
	CacheHitRate    float64       `json:"cache_hit_rate"`
	Duration        time.Duration `json:"duration"`
	Timestamp       time.Time     `json:"timestamp"`
}

// BenchmarkResultJSON is for JSON serialization with string durations
type BenchmarkResultJSON struct {
	Strategy        string  `json:"strategy"`
	TotalTweets     int     `json:"total_tweets"`
	TotalReads      int     `json:"total_reads"`
	WriteLatencyP50 string  `json:"write_latency_p50"`
	WriteLatencyP95 string  `json:"write_latency_p95"`
	WriteLatencyP99 string  `json:"write_latency_p99"`
	WriteLatencyAvg string  `json:"write_latency_avg"`
	ReadLatencyP50  string  `json:"read_latency_p50"`
	ReadLatencyP95  string  `json:"read_latency_p95"`
	ReadLatencyP99  string  `json:"read_latency_p99"`
	ReadLatencyAvg  string  `json:"read_latency_avg"`
	WriteThroughput float64 `json:"write_throughput"`
	ReadThroughput  float64 `json:"read_throughput"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	Duration        string  `json:"duration"`
	Timestamp       string  `json:"timestamp"`
}

// ToJSON converts BenchmarkResult to JSON-friendly format
func (b *BenchmarkResult) ToJSON() BenchmarkResultJSON {
	return BenchmarkResultJSON{
		Strategy:        b.Strategy,
		TotalTweets:     b.TotalTweets,
		TotalReads:      b.TotalReads,
		WriteLatencyP50: b.WriteLatencyP50.String(),
		WriteLatencyP95: b.WriteLatencyP95.String(),
		WriteLatencyP99: b.WriteLatencyP99.String(),
		WriteLatencyAvg: b.WriteLatencyAvg.String(),
		ReadLatencyP50:  b.ReadLatencyP50.String(),
		ReadLatencyP95:  b.ReadLatencyP95.String(),
		ReadLatencyP99:  b.ReadLatencyP99.String(),
		ReadLatencyAvg:  b.ReadLatencyAvg.String(),
		WriteThroughput: b.WriteThroughput,
		ReadThroughput:  b.ReadThroughput,
		CacheHitRate:    b.CacheHitRate,
		Duration:        b.Duration.String(),
		Timestamp:       b.Timestamp.Format(time.RFC3339),
	}
}

// Metrics holds real-time metrics for the dashboard
type Metrics struct {
	ActiveStrategy     string             `json:"active_strategy"`
	TotalUsers         int                `json:"total_users"`
	TotalTweets        int                `json:"total_tweets"`
	TotalFollows       int                `json:"total_follows"`
	CelebrityCount     int                `json:"celebrity_count"`
	RedisMemoryUsage   int64              `json:"redis_memory_usage"`
	PostgresSize       int64              `json:"postgres_size"`
	RecentWriteLatency []LatencyDataPoint `json:"recent_write_latency"`
	RecentReadLatency  []LatencyDataPoint `json:"recent_read_latency"`
}

// LatencyDataPoint represents a single latency measurement
type LatencyDataPoint struct {
	Timestamp time.Time     `json:"timestamp"`
	Latency   time.Duration `json:"latency"`
	Strategy  string        `json:"strategy"`
}

// SeedConfig holds configuration for data seeding
type SeedConfig struct {
	UserCount       int     `json:"user_count"`
	AvgFollowers    int     `json:"avg_followers"`
	CelebrityCount  int     `json:"celebrity_count"`
	TweetsPerUser   int     `json:"tweets_per_user"`
	FollowerStdDev  float64 `json:"follower_std_dev"`
}

// DefaultSeedConfig returns default seeding configuration
func DefaultSeedConfig() SeedConfig {
	return SeedConfig{
		UserCount:      10000,
		AvgFollowers:   150,
		CelebrityCount: 50,
		TweetsPerUser:  10,
		FollowerStdDev: 100,
	}
}
