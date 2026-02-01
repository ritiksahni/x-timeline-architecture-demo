package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ritik/twitter-fan-out/internal/cache"
	"github.com/ritik/twitter-fan-out/internal/config"
	"github.com/ritik/twitter-fan-out/internal/models"
	"github.com/ritik/twitter-fan-out/internal/repository"
	"github.com/ritik/twitter-fan-out/internal/timeline"
	"github.com/spf13/cobra"
)

var (
	benchStrategy   string
	benchTweets     int
	benchReads      int
	benchConcurrent int
	benchDuration   time.Duration
	benchOutput     string
)

func init() {
	benchmarkCmd.Flags().StringVar(&benchStrategy, "strategy", "all", "Strategy to benchmark (fanout_write, fanout_read, hybrid, all)")
	benchmarkCmd.Flags().IntVar(&benchTweets, "tweets", 1000, "Number of tweets to post")
	benchmarkCmd.Flags().IntVar(&benchReads, "reads", 2000, "Number of timeline reads")
	benchmarkCmd.Flags().IntVar(&benchConcurrent, "concurrent", 50, "Number of concurrent workers")
	benchmarkCmd.Flags().DurationVar(&benchDuration, "duration", 0, "Duration to run (overrides tweet count)")
	benchmarkCmd.Flags().StringVar(&benchOutput, "output", "", "Output file for results (JSON)")
	
	rootCmd.AddCommand(benchmarkCmd)
}

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks",
	Long: `Run benchmarks comparing the three timeline strategies.

Measures:
  - Write latency (posting tweets)
  - Read latency (fetching timelines)
  - Throughput (operations per second)
  - Fan-out time for high-follower users`,
	Run: runBenchmark,
}

func runBenchmark(cmd *cobra.Command, args []string) {
	fmt.Println("🏃 Running benchmarks...")
	fmt.Printf("   Strategy: %s\n", benchStrategy)
	fmt.Printf("   Tweets: %d\n", benchTweets)
	fmt.Printf("   Reads: %d\n", benchReads)
	fmt.Printf("   Concurrent: %d\n", benchConcurrent)
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

	// Initialize Redis
	redisClient, err := cache.InitRedis(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		os.Exit(1)
	}
	defer cache.Close()

	// Create repositories
	userRepo := repository.NewUserRepository(db)
	tweetRepo := repository.NewTweetRepository(db)
	followRepo := repository.NewFollowRepository(db)

	// Create cache
	timelineCache := cache.NewTimelineCache(redisClient, cfg.TimelineCacheSize)

	// Get users for benchmarking
	users, err := userRepo.GetRandomUsers(ctx, 1000)
	if err != nil || len(users) == 0 {
		fmt.Printf("❌ No users found. Run 'fanout seed' first.\n")
		os.Exit(1)
	}

	fmt.Printf("📊 Found %d users for benchmarking\n\n", len(users))

	var results []*models.BenchmarkResult

	strategies := []string{benchStrategy}
	if benchStrategy == "all" {
		strategies = []string{"fanout_write", "fanout_read", "hybrid"}
	}

	for _, strategyName := range strategies {
		var strategy timeline.Strategy

		switch strategyName {
		case "fanout_write":
			strategy = timeline.NewFanOutWriteStrategy(tweetRepo, followRepo, userRepo, timelineCache)
		case "fanout_read":
			strategy = timeline.NewFanOutReadStrategy(tweetRepo, followRepo, userRepo, timelineCache)
		case "hybrid":
			strategy = timeline.NewHybridStrategy(tweetRepo, followRepo, userRepo, timelineCache, cfg.CelebrityThreshold)
		default:
			fmt.Printf("❌ Unknown strategy: %s\n", strategyName)
			continue
		}

		result := runStrategyBenchmark(ctx, strategy, users, benchTweets, benchReads, benchConcurrent)
		results = append(results, result)
	}

	// Print results
	printResults(results)

	// Save to file if requested
	if benchOutput != "" {
		saveResults(results, benchOutput)
	}
}

func runStrategyBenchmark(ctx context.Context, strategy timeline.Strategy, users []*models.User, numTweets, numReads, concurrent int) *models.BenchmarkResult {
	fmt.Printf("📈 Benchmarking %s...\n", strategy.Name())

	result := &models.BenchmarkResult{
		Strategy:    strategy.Name(),
		TotalTweets: numTweets,
		TotalReads:  numReads,
		Timestamp:   time.Now(),
	}

	// Benchmark writes
	fmt.Printf("   Writing %d tweets with %d workers...\n", numTweets, concurrent)
	writeLatencies := benchmarkWrites(ctx, strategy, users, numTweets, concurrent)
	
	// Benchmark reads
	fmt.Printf("   Reading %d timelines with %d workers...\n", numReads, concurrent)
	readLatencies, cacheHits := benchmarkReads(ctx, strategy, users, numReads, concurrent)

	// Calculate statistics
	result.WriteLatencyP50 = percentile(writeLatencies, 50)
	result.WriteLatencyP95 = percentile(writeLatencies, 95)
	result.WriteLatencyP99 = percentile(writeLatencies, 99)
	result.WriteLatencyAvg = avg(writeLatencies)

	result.ReadLatencyP50 = percentile(readLatencies, 50)
	result.ReadLatencyP95 = percentile(readLatencies, 95)
	result.ReadLatencyP99 = percentile(readLatencies, 99)
	result.ReadLatencyAvg = avg(readLatencies)

	totalWriteTime := sum(writeLatencies)
	totalReadTime := sum(readLatencies)
	
	result.WriteThroughput = float64(numTweets) / totalWriteTime.Seconds() * float64(concurrent)
	result.ReadThroughput = float64(numReads) / totalReadTime.Seconds() * float64(concurrent)
	result.CacheHitRate = float64(cacheHits) / float64(numReads)
	result.Duration = totalWriteTime + totalReadTime

	fmt.Printf("   ✓ Complete\n\n")

	return result
}

func benchmarkWrites(ctx context.Context, strategy timeline.Strategy, users []*models.User, count, concurrent int) []time.Duration {
	latencies := make([]time.Duration, 0, count)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	tweetsPerWorker := count / concurrent
	var completed int64

	sampleContent := []string{
		"Benchmark tweet #1",
		"Testing the system",
		"Performance test in progress",
		"Just another tweet",
		"Measuring latency",
	}

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < tweetsPerWorker; j++ {
				user := users[rand.Intn(len(users))]
				content := sampleContent[rand.Intn(len(sampleContent))]
				
				start := time.Now()
				_, _, err := strategy.PostTweet(ctx, user.ID, content)
				elapsed := time.Since(start)
				
				if err == nil {
					mu.Lock()
					latencies = append(latencies, elapsed)
					mu.Unlock()
				}
				
				c := atomic.AddInt64(&completed, 1)
				if c%100 == 0 {
					fmt.Printf("   Progress: %d/%d tweets\r", c, count)
				}
			}
		}()
	}

	wg.Wait()
	fmt.Printf("   Progress: %d/%d tweets\n", count, count)
	
	return latencies
}

func benchmarkReads(ctx context.Context, strategy timeline.Strategy, users []*models.User, count, concurrent int) ([]time.Duration, int) {
	latencies := make([]time.Duration, 0, count)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var cacheHits int64
	
	readsPerWorker := count / concurrent
	var completed int64

	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < readsPerWorker; j++ {
				user := users[rand.Intn(len(users))]
				
				start := time.Now()
				_, metrics, err := strategy.GetTimeline(ctx, user.ID, 50, 0)
				elapsed := time.Since(start)
				
				if err == nil {
					mu.Lock()
					latencies = append(latencies, elapsed)
					mu.Unlock()
					
					if metrics != nil && metrics.CacheHit {
						atomic.AddInt64(&cacheHits, 1)
					}
				}
				
				c := atomic.AddInt64(&completed, 1)
				if c%100 == 0 {
					fmt.Printf("   Progress: %d/%d reads\r", c, count)
				}
			}
		}()
	}

	wg.Wait()
	fmt.Printf("   Progress: %d/%d reads\n", count, count)
	
	return latencies, int(cacheHits)
}

func printResults(results []*models.BenchmarkResult) {
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("                        BENCHMARK RESULTS                           ")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Header
	fmt.Printf("%-15s │ %-12s │ %-12s │ %-12s │ %-10s\n", 
		"Strategy", "Write P50", "Write P95", "Read P50", "Read P95")
	fmt.Println("────────────────┼──────────────┼──────────────┼──────────────┼────────────")

	for _, r := range results {
		fmt.Printf("%-15s │ %-12s │ %-12s │ %-12s │ %-10s\n",
			r.Strategy,
			r.WriteLatencyP50.Round(time.Microsecond),
			r.WriteLatencyP95.Round(time.Microsecond),
			r.ReadLatencyP50.Round(time.Microsecond),
			r.ReadLatencyP95.Round(time.Microsecond),
		)
	}

	fmt.Println()
	fmt.Println("Throughput & Cache:")
	fmt.Printf("%-15s │ %-15s │ %-15s │ %-12s\n", 
		"Strategy", "Write/sec", "Read/sec", "Cache Hit %")
	fmt.Println("────────────────┼─────────────────┼─────────────────┼──────────────")

	for _, r := range results {
		fmt.Printf("%-15s │ %-15.1f │ %-15.1f │ %-12.1f%%\n",
			r.Strategy,
			r.WriteThroughput,
			r.ReadThroughput,
			r.CacheHitRate*100,
		)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
}

func saveResults(results []*models.BenchmarkResult, filename string) {
	jsonResults := make([]models.BenchmarkResultJSON, len(results))
	for i, r := range results {
		jsonResults[i] = r.ToJSON()
	}

	data, err := json.MarshalIndent(jsonResults, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal results: %v\n", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		fmt.Printf("❌ Failed to write results: %v\n", err)
		return
	}

	fmt.Printf("📄 Results saved to %s\n", filename)
}

// Helper functions
func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func avg(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func sum(durations []time.Duration) time.Duration {
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total
}
