package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/ritik/twitter-fan-out/internal/cache"
	"github.com/ritik/twitter-fan-out/internal/config"
	"github.com/ritik/twitter-fan-out/internal/repository"
	"github.com/spf13/cobra"
)

var (
	seedUsers        int
	seedAvgFollowers int
	seedCelebrities  int
	seedTweetsPerUser int
	seedClear        bool
)

func init() {
	seedCmd.Flags().IntVar(&seedUsers, "users", 10000, "Number of users to create")
	seedCmd.Flags().IntVar(&seedAvgFollowers, "avg-followers", 150, "Average followers per user")
	seedCmd.Flags().IntVar(&seedCelebrities, "celebrities", 50, "Number of celebrity users")
	seedCmd.Flags().IntVar(&seedTweetsPerUser, "tweets-per-user", 10, "Tweets per user")
	seedCmd.Flags().BoolVar(&seedClear, "clear", false, "Clear existing data before seeding")
	
	rootCmd.AddCommand(seedCmd)
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with test data",
	Long: `Generate test users, follows, and tweets for benchmarking.

This creates a realistic social graph with:
  - Regular users with varying follower counts
  - Celebrity users with high follower counts
  - Follow relationships following a power-law distribution
  - Sample tweets for each user`,
	Run: runSeed,
}

func runSeed(cmd *cobra.Command, args []string) {
	fmt.Println("🌱 Seeding database...")
	fmt.Printf("   Users: %d\n", seedUsers)
	fmt.Printf("   Avg followers: %d\n", seedAvgFollowers)
	fmt.Printf("   Celebrities: %d\n", seedCelebrities)
	fmt.Printf("   Tweets per user: %d\n", seedTweetsPerUser)
	fmt.Println()

	cfg := config.Get()
	ctx := context.Background()

	// Initialize database
	db, err := repository.InitDB(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer repository.Close()

	// Run migrations
	if err := repository.RunMigrations(db, "migrations"); err != nil {
		fmt.Printf("⚠️  Warning: Failed to run migrations: %v\n", err)
	}

	// Initialize Redis
	_, err = cache.InitRedis(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		os.Exit(1)
	}
	defer cache.Close()

	// Create repositories
	userRepo := repository.NewUserRepository(db)
	tweetRepo := repository.NewTweetRepository(db)
	followRepo := repository.NewFollowRepository(db)

	// Clear existing data if requested
	if seedClear {
		fmt.Println("🗑️  Clearing existing data...")
		followRepo.Truncate(ctx)
		tweetRepo.Truncate(ctx)
		userRepo.Truncate(ctx)
		cache.FlushAll(ctx)
		fmt.Println("   Done")
	}

	// Seed users
	fmt.Printf("👤 Creating %d users...\n", seedUsers)
	start := time.Now()
	
	usernames := make([]string, seedUsers)
	for i := 0; i < seedUsers; i++ {
		usernames[i] = fmt.Sprintf("user_%d", i+1)
	}
	
	// Batch create users
	batchSize := 1000
	for i := 0; i < len(usernames); i += batchSize {
		end := i + batchSize
		if end > len(usernames) {
			end = len(usernames)
		}
		if err := userRepo.BulkCreate(ctx, usernames[i:end]); err != nil {
			fmt.Printf("❌ Failed to create users: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("   Created %d/%d users\r", end, seedUsers)
	}
	fmt.Printf("   Created %d users in %v\n", seedUsers, time.Since(start))

	// Get all users for follow relationships
	users, err := userRepo.GetAll(ctx, seedUsers, 0)
	if err != nil {
		fmt.Printf("❌ Failed to get users: %v\n", err)
		os.Exit(1)
	}

	// Create follow relationships
	fmt.Printf("🔗 Creating follow relationships...\n")
	start = time.Now()
	
	// Celebrity users get more followers
	celebrityIDs := make(map[int64]bool)
	for i := 0; i < seedCelebrities && i < len(users); i++ {
		celebrityIDs[users[i].ID] = true
	}

	// Generate follows with power-law distribution
	follows := make([]struct {
		FollowerID int64
		FolloweeID int64
	}, 0)

	totalFollows := seedUsers * seedAvgFollowers
	
	for i := 0; i < totalFollows; i++ {
		followerIdx := rand.Intn(len(users))
		
		// Celebrities have higher chance of being followed
		var followeeIdx int
		if rand.Float64() < 0.3 && seedCelebrities > 0 { // 30% chance to follow a celebrity
			followeeIdx = rand.Intn(seedCelebrities)
		} else {
			followeeIdx = rand.Intn(len(users))
		}
		
		// Don't follow yourself
		if followerIdx == followeeIdx {
			continue
		}
		
		follows = append(follows, struct {
			FollowerID int64
			FolloweeID int64
		}{
			FollowerID: users[followerIdx].ID,
			FolloweeID: users[followeeIdx].ID,
		})
	}

	// Batch create follows
	for i := 0; i < len(follows); i += batchSize {
		end := i + batchSize
		if end > len(follows) {
			end = len(follows)
		}
		if err := followRepo.BulkCreate(ctx, follows[i:end]); err != nil {
			fmt.Printf("⚠️  Warning: Some follows failed: %v\n", err)
		}
		fmt.Printf("   Created %d/%d follows\r", end, len(follows))
	}
	fmt.Printf("   Created %d follows in %v\n", len(follows), time.Since(start))

	// Create tweets
	fmt.Printf("📝 Creating tweets...\n")
	start = time.Now()
	
	sampleTweets := []string{
		"Just had the best coffee! ☕",
		"Working on something exciting...",
		"Beautiful day outside! 🌞",
		"Can't believe this happened today",
		"Learning new things every day",
		"Just finished a great book 📚",
		"Thinking about the future...",
		"Great meeting with the team today",
		"Weekend vibes! 🎉",
		"Grateful for all the support",
		"New project coming soon!",
		"Just hit a major milestone 🎯",
		"Coffee and code, perfect combo",
		"Exploring new ideas today",
		"Thankful for this community",
	}

	tweets := make([]struct {
		UserID  int64
		Content string
	}, 0)

	for _, user := range users {
		for j := 0; j < seedTweetsPerUser; j++ {
			content := sampleTweets[rand.Intn(len(sampleTweets))]
			tweets = append(tweets, struct {
				UserID  int64
				Content string
			}{
				UserID:  user.ID,
				Content: content,
			})
		}
	}

	// Batch create tweets
	for i := 0; i < len(tweets); i += batchSize {
		end := i + batchSize
		if end > len(tweets) {
			end = len(tweets)
		}
		if err := tweetRepo.BulkCreate(ctx, tweets[i:end]); err != nil {
			fmt.Printf("⚠️  Warning: Some tweets failed: %v\n", err)
		}
		fmt.Printf("   Created %d/%d tweets\r", end, len(tweets))
	}
	fmt.Printf("   Created %d tweets in %v\n", len(tweets), time.Since(start))

	// Print summary
	fmt.Println()
	fmt.Println("✅ Seeding complete!")
	fmt.Println()
	
	// Get actual counts
	userCount, _ := userRepo.Count(ctx)
	tweetCount, _ := tweetRepo.Count(ctx)
	followCount, _ := followRepo.Count(ctx)
	celebrityCount, _ := userRepo.CountCelebrities(ctx, cfg.CelebrityThreshold)
	
	fmt.Println("📊 Database Statistics:")
	fmt.Printf("   Total users:      %d\n", userCount)
	fmt.Printf("   Total tweets:     %d\n", tweetCount)
	fmt.Printf("   Total follows:    %d\n", followCount)
	fmt.Printf("   Celebrities:      %d (>= %d followers)\n", celebrityCount, cfg.CelebrityThreshold)
}
