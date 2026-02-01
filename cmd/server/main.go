package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ritik/twitter-fan-out/internal/api"
	"github.com/ritik/twitter-fan-out/internal/cache"
	"github.com/ritik/twitter-fan-out/internal/config"
	"github.com/ritik/twitter-fan-out/internal/repository"
	"github.com/ritik/twitter-fan-out/internal/timeline"
)

func main() {
	// Load configuration
	cfg := config.Get()

	// Initialize database
	db, err := repository.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer repository.Close()

	// Run migrations
	if err := repository.RunMigrations(db, "migrations"); err != nil {
		log.Printf("Warning: Failed to run migrations: %v", err)
	}

	// Initialize Redis
	redisClient, err := cache.InitRedis(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer cache.Close()

	// Create repositories
	userRepo := repository.NewUserRepository(db)
	tweetRepo := repository.NewTweetRepository(db)
	followRepo := repository.NewFollowRepository(db)

	// Create cache
	timelineCache := cache.NewTimelineCache(redisClient, cfg.TimelineCacheSize)

	// Create timeline strategies
	fanOutWrite := timeline.NewFanOutWriteStrategy(tweetRepo, followRepo, userRepo, timelineCache)
	fanOutRead := timeline.NewFanOutReadStrategy(tweetRepo, followRepo, userRepo, timelineCache)
	hybrid := timeline.NewHybridStrategy(tweetRepo, followRepo, userRepo, timelineCache, cfg.CelebrityThreshold)

	// Create API handler
	handler := api.NewHandler(cfg, fanOutWrite, fanOutRead, hybrid, userRepo, followRepo)

	// Create router
	router := api.NewRouter(handler)

	// Create server
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 Server starting on http://localhost:%s\n", cfg.ServerPort)
		fmt.Printf("   Celebrity threshold: %d followers\n", cfg.CelebrityThreshold)
		fmt.Printf("   Timeline cache size: %d tweets\n", cfg.TimelineCacheSize)
		fmt.Println()
		fmt.Println("Available endpoints:")
		fmt.Println("   POST /api/tweet              - Post a tweet")
		fmt.Println("   GET  /api/timeline/{user_id} - Get user timeline")
		fmt.Println("   GET  /api/config             - Get configuration")
		fmt.Println("   PUT  /api/config             - Update configuration")
		fmt.Println("   GET  /api/metrics            - Get metrics summary")
		fmt.Println("   GET  /api/metrics/recent     - Get recent metrics")
		fmt.Println("   GET  /health                 - Health check")
		fmt.Println()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	fmt.Println("Server stopped")
}
