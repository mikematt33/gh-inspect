package cli

import (
	"fmt"
	"os"

	"github.com/mikematt33/gh-inspect/internal/cache"
	"github.com/spf13/cobra"
)

var (
	flagClearStats bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the API response cache",
	Long: `Manage the disk-based cache for GitHub API responses.
The cache stores API responses locally to reduce API rate limit usage and speed up repeated analyses.
Cached data expires after 1 hour by default.`,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached API responses",
	Long: `Remove all cached GitHub API responses from disk.
This forces fresh API calls on the next analysis run.`,
	Example: `  gh-inspect cache clear
  gh-inspect cache clear --stats`,
	Run: runCacheClear,
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  `Display information about the current cache including entry count and total size.`,
	Run:   runCacheStats,
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheStatsCmd)

	cacheClearCmd.Flags().BoolVar(&flagClearStats, "stats", false, "Show statistics before clearing")
}

func runCacheClear(cmd *cobra.Command, args []string) {
	cachePath, err := cache.GetDefaultCachePath()
	if err != nil {
		fmt.Printf("Error getting cache path: %v\n", err)
		os.Exit(1)
	}

	c, err := cache.New(cachePath, 0)
	if err != nil {
		fmt.Printf("Error initializing cache: %v\n", err)
		os.Exit(1)
	}

	// Show stats before clearing if requested
	if flagClearStats {
		count, size, err := c.Stats()
		if err != nil {
			fmt.Printf("Error getting cache stats: %v\n", err)
		} else {
			fmt.Printf("Cache statistics before clearing:\n")
			fmt.Printf("  Entries: %d\n", count)
			fmt.Printf("  Size: %.2f MB\n", float64(size)/(1024*1024))
		}
	}

	if err := c.Clear(); err != nil {
		fmt.Printf("Error clearing cache: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Cache cleared successfully")
}

func runCacheStats(cmd *cobra.Command, args []string) {
	cachePath, err := cache.GetDefaultCachePath()
	if err != nil {
		fmt.Printf("Error getting cache path: %v\n", err)
		os.Exit(1)
	}

	c, err := cache.New(cachePath, 0)
	if err != nil {
		fmt.Printf("Error initializing cache: %v\n", err)
		os.Exit(1)
	}

	count, size, err := c.Stats()
	if err != nil {
		fmt.Printf("Error getting cache stats: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cache statistics:\n")
	fmt.Printf("  Location: %s\n", cachePath)
	fmt.Printf("  Entries: %d\n", count)
	fmt.Printf("  Size: %.2f MB\n", float64(size)/(1024*1024))
	fmt.Printf("  TTL: 1 hour\n")
}
