package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/ritik/twitter-fan-out/internal/config"
)

var client *redis.Client

// InitRedis initializes the Redis connection
func InitRedis(cfg *config.Config) (*redis.Client, error) {
	client = redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}

// GetClient returns the Redis client
func GetClient() *redis.Client {
	return client
}

// Close closes the Redis connection
func Close() error {
	if client != nil {
		return client.Close()
	}
	return nil
}

// FlushAll clears all data from Redis (for testing/reset)
func FlushAll(ctx context.Context) error {
	return client.FlushAll(ctx).Err()
}

// GetMemoryUsage returns Redis memory usage in bytes
func GetMemoryUsage(ctx context.Context) (int64, error) {
	info, err := client.Info(ctx, "memory").Result()
	if err != nil {
		return 0, err
	}
	
	// Parse used_memory from info string
	var usedMemory int64
	_, err = fmt.Sscanf(info, "# Memory\r\nused_memory:%d", &usedMemory)
	if err != nil {
		// Try alternative parsing
		return 0, nil
	}
	return usedMemory, nil
}
