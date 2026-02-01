package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "fanout",
	Short: "Twitter Fan-Out Timeline Prototype CLI",
	Long: `A CLI tool for managing and benchmarking the Twitter Fan-Out Timeline Prototype.

This tool allows you to:
  - Configure the system (celebrity threshold, cache sizes)
  - Seed the database with test data
  - Run benchmarks comparing different timeline strategies
  - View benchmark results`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
