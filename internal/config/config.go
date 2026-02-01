package config

import (
	"encoding/json"
	"os"
	"sync"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	ServerPort string `json:"server_port"`

	// Database settings
	PostgresHost     string `json:"postgres_host"`
	PostgresPort     string `json:"postgres_port"`
	PostgresUser     string `json:"postgres_user"`
	PostgresPassword string `json:"postgres_password"`
	PostgresDB       string `json:"postgres_db"`

	// Redis settings
	RedisHost     string `json:"redis_host"`
	RedisPort     string `json:"redis_port"`
	RedisPassword string `json:"redis_password"`
	RedisDB       int    `json:"redis_db"`

	// Timeline settings
	CelebrityThreshold int `json:"celebrity_threshold"` // Follower count above which user is considered celebrity
	TimelineCacheSize  int `json:"timeline_cache_size"` // Max tweets to keep in timeline cache
	TimelinePageSize   int `json:"timeline_page_size"`  // Default page size for timeline queries

	// Benchmark settings
	BenchmarkTweets     int `json:"benchmark_tweets"`
	BenchmarkConcurrent int `json:"benchmark_concurrent"`
}

var (
	instance *Config
	once     sync.Once
)

// Default returns the default configuration
func Default() *Config {
	return &Config{
		ServerPort:         "8080",
		PostgresHost:       "localhost",
		PostgresPort:       "5432",
		PostgresUser:       "fanout",
		PostgresPassword:   "fanout",
		PostgresDB:         "fanout",
		RedisHost:          "localhost",
		RedisPort:          "6379",
		RedisPassword:      "",
		RedisDB:            0,
		CelebrityThreshold: 10000,
		TimelineCacheSize:  800,
		TimelinePageSize:   50,
		BenchmarkTweets:    1000,
		BenchmarkConcurrent: 50,
	}
}

// Get returns the singleton config instance
func Get() *Config {
	once.Do(func() {
		instance = Default()
		instance.loadFromEnv()
	})
	return instance
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() {
	if v := os.Getenv("SERVER_PORT"); v != "" {
		c.ServerPort = v
	}
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		c.PostgresHost = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		c.PostgresPort = v
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		c.PostgresUser = v
	}
	if v := os.Getenv("POSTGRES_PASSWORD"); v != "" {
		c.PostgresPassword = v
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		c.PostgresDB = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		c.RedisHost = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		c.RedisPort = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		c.RedisPassword = v
	}
}

// PostgresDSN returns the PostgreSQL connection string
func (c *Config) PostgresDSN() string {
	return "host=" + c.PostgresHost +
		" port=" + c.PostgresPort +
		" user=" + c.PostgresUser +
		" password=" + c.PostgresPassword +
		" dbname=" + c.PostgresDB +
		" sslmode=disable"
}

// RedisAddr returns the Redis address
func (c *Config) RedisAddr() string {
	return c.RedisHost + ":" + c.RedisPort
}

// SaveToFile saves the current config to a JSON file
func (c *Config) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadFromFile loads config from a JSON file
func (c *Config) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

// Update updates specific config values
func (c *Config) Update(key string, value interface{}) {
	switch key {
	case "celebrity_threshold", "celebrity-threshold":
		if v, ok := value.(int); ok {
			c.CelebrityThreshold = v
		}
	case "timeline_cache_size", "timeline-cache-size":
		if v, ok := value.(int); ok {
			c.TimelineCacheSize = v
		}
	case "timeline_page_size", "timeline-page-size":
		if v, ok := value.(int); ok {
			c.TimelinePageSize = v
		}
	}
}
