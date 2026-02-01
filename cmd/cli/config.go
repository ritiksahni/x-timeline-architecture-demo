package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/ritik/twitter-fan-out/internal/config"
	"github.com/spf13/cobra"
)

var configFile string

func init() {
	configCmd.PersistentFlags().StringVarP(&configFile, "file", "f", "config.json", "Config file path")
	
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and modify configuration settings for the fan-out prototype.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		
		// Try to load from file if exists
		if _, err := os.Stat(configFile); err == nil {
			cfg.LoadFromFile(configFile)
		}
		
		fmt.Println("Current Configuration:")
		fmt.Println("======================")
		fmt.Printf("Server Port:          %s\n", cfg.ServerPort)
		fmt.Printf("PostgreSQL Host:      %s:%s\n", cfg.PostgresHost, cfg.PostgresPort)
		fmt.Printf("PostgreSQL Database:  %s\n", cfg.PostgresDB)
		fmt.Printf("Redis Host:           %s:%s\n", cfg.RedisHost, cfg.RedisPort)
		fmt.Println()
		fmt.Println("Timeline Settings:")
		fmt.Printf("  Celebrity Threshold:  %d followers\n", cfg.CelebrityThreshold)
		fmt.Printf("  Timeline Cache Size:  %d tweets\n", cfg.TimelineCacheSize)
		fmt.Printf("  Timeline Page Size:   %d tweets\n", cfg.TimelinePageSize)
		fmt.Println()
		fmt.Println("Benchmark Settings:")
		fmt.Printf("  Default Tweets:       %d\n", cfg.BenchmarkTweets)
		fmt.Printf("  Default Concurrent:   %d\n", cfg.BenchmarkConcurrent)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		
		// Try to load from file if exists
		if _, err := os.Stat(configFile); err == nil {
			cfg.LoadFromFile(configFile)
		}
		
		key := args[0]
		var value interface{}
		
		switch key {
		case "celebrity-threshold", "celebrity_threshold":
			value = cfg.CelebrityThreshold
		case "timeline-cache-size", "timeline_cache_size":
			value = cfg.TimelineCacheSize
		case "timeline-page-size", "timeline_page_size":
			value = cfg.TimelinePageSize
		case "server-port", "server_port":
			value = cfg.ServerPort
		case "postgres-host", "postgres_host":
			value = cfg.PostgresHost
		case "redis-host", "redis_host":
			value = cfg.RedisHost
		default:
			fmt.Printf("Unknown config key: %s\n", key)
			fmt.Println("\nAvailable keys:")
			fmt.Println("  celebrity-threshold")
			fmt.Println("  timeline-cache-size")
			fmt.Println("  timeline-page-size")
			fmt.Println("  server-port")
			fmt.Println("  postgres-host")
			fmt.Println("  redis-host")
			os.Exit(1)
		}
		
		fmt.Printf("%s = %v\n", key, value)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		
		// Try to load from file if exists
		if _, err := os.Stat(configFile); err == nil {
			cfg.LoadFromFile(configFile)
		}
		
		key := args[0]
		valueStr := args[1]
		
		switch key {
		case "celebrity-threshold", "celebrity_threshold":
			value, err := strconv.Atoi(valueStr)
			if err != nil {
				fmt.Printf("Invalid value for %s: %s (must be integer)\n", key, valueStr)
				os.Exit(1)
			}
			cfg.CelebrityThreshold = value
			
		case "timeline-cache-size", "timeline_cache_size":
			value, err := strconv.Atoi(valueStr)
			if err != nil {
				fmt.Printf("Invalid value for %s: %s (must be integer)\n", key, valueStr)
				os.Exit(1)
			}
			cfg.TimelineCacheSize = value
			
		case "timeline-page-size", "timeline_page_size":
			value, err := strconv.Atoi(valueStr)
			if err != nil {
				fmt.Printf("Invalid value for %s: %s (must be integer)\n", key, valueStr)
				os.Exit(1)
			}
			cfg.TimelinePageSize = value
			
		case "server-port", "server_port":
			cfg.ServerPort = valueStr
			
		default:
			fmt.Printf("Unknown or read-only config key: %s\n", key)
			os.Exit(1)
		}
		
		// Save to file
		if err := cfg.SaveToFile(configFile); err != nil {
			fmt.Printf("Failed to save config: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Set %s = %s\n", key, valueStr)
		fmt.Printf("Config saved to %s\n", configFile)
	},
}

// Helper to pretty print config as JSON
func printConfigJSON(cfg *config.Config) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
