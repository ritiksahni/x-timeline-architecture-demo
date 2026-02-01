package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ritik/twitter-fan-out/internal/models"
	"github.com/spf13/cobra"
)

var (
	resultsFormat string
	resultsInput  string
)

func init() {
	resultsCmd.Flags().StringVar(&resultsFormat, "format", "table", "Output format (table, json)")
	resultsCmd.Flags().StringVar(&resultsInput, "input", "benchmark_results.json", "Input file with benchmark results")
	
	rootCmd.AddCommand(resultsCmd)
}

var resultsCmd = &cobra.Command{
	Use:   "results",
	Short: "View benchmark results",
	Long:  `Display benchmark results from a previous run.`,
	Run:   runResults,
}

func runResults(cmd *cobra.Command, args []string) {
	// Read results file
	data, err := os.ReadFile(resultsInput)
	if err != nil {
		fmt.Printf("❌ Failed to read results file: %v\n", err)
		fmt.Println("   Run 'fanout benchmark --output benchmark_results.json' first")
		os.Exit(1)
	}

	var results []models.BenchmarkResultJSON
	if err := json.Unmarshal(data, &results); err != nil {
		fmt.Printf("❌ Failed to parse results: %v\n", err)
		os.Exit(1)
	}

	switch resultsFormat {
	case "json":
		printResultsJSON(results)
	case "table":
		printResultsTable(results)
	default:
		fmt.Printf("❌ Unknown format: %s\n", resultsFormat)
		os.Exit(1)
	}
}

func printResultsJSON(results []models.BenchmarkResultJSON) {
	data, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(data))
}

func printResultsTable(results []models.BenchmarkResultJSON) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("                           BENCHMARK RESULTS                                ")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	for _, r := range results {
		fmt.Printf("Strategy: %s\n", r.Strategy)
		fmt.Println("───────────────────────────────────────────────────────────────────────────")
		fmt.Printf("  Total Tweets:     %d\n", r.TotalTweets)
		fmt.Printf("  Total Reads:      %d\n", r.TotalReads)
		fmt.Printf("  Duration:         %s\n", r.Duration)
		fmt.Println()
		fmt.Println("  Write Latency:")
		fmt.Printf("    P50: %s\n", r.WriteLatencyP50)
		fmt.Printf("    P95: %s\n", r.WriteLatencyP95)
		fmt.Printf("    P99: %s\n", r.WriteLatencyP99)
		fmt.Printf("    Avg: %s\n", r.WriteLatencyAvg)
		fmt.Println()
		fmt.Println("  Read Latency:")
		fmt.Printf("    P50: %s\n", r.ReadLatencyP50)
		fmt.Printf("    P95: %s\n", r.ReadLatencyP95)
		fmt.Printf("    P99: %s\n", r.ReadLatencyP99)
		fmt.Printf("    Avg: %s\n", r.ReadLatencyAvg)
		fmt.Println()
		fmt.Println("  Throughput:")
		fmt.Printf("    Writes/sec: %.1f\n", r.WriteThroughput)
		fmt.Printf("    Reads/sec:  %.1f\n", r.ReadThroughput)
		fmt.Println()
		fmt.Printf("  Cache Hit Rate: %.1f%%\n", r.CacheHitRate*100)
		fmt.Println()
	}

	// Comparison summary
	if len(results) > 1 {
		fmt.Println("═══════════════════════════════════════════════════════════════════════════")
		fmt.Println("                              COMPARISON                                    ")
		fmt.Println("═══════════════════════════════════════════════════════════════════════════")
		fmt.Println()
		fmt.Printf("%-15s │ %-12s │ %-12s │ %-12s │ %-10s\n", 
			"Strategy", "Write P95", "Read P95", "Write/s", "Read/s")
		fmt.Println("────────────────┼──────────────┼──────────────┼──────────────┼────────────")
		
		for _, r := range results {
			fmt.Printf("%-15s │ %-12s │ %-12s │ %-12.0f │ %-10.0f\n",
				r.Strategy,
				r.WriteLatencyP95,
				r.ReadLatencyP95,
				r.WriteThroughput,
				r.ReadThroughput,
			)
		}
		fmt.Println()
	}
}
