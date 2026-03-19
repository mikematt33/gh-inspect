package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/spf13/cobra"
)

const defaultConfig = `# gh-inspect Configuration

# Global settings
global:
  concurrency: 5          # Max concurrent repo analysis
  output_mode: "observational" # How findings are presented: observational (default), suggestive, statistical
  # github_token: "YOUR_TOKEN" # Optional: store token here (not recommended for shared machines)

# Analyzer configuration
# Enable or disable specific analyzers and tune their parameters
analyzers:
  pr_flow:
    enabled: true
    params:
      stale_threshold_days: 14  # PRs inactive longer than this are flagged

  issue_hygiene:
    enabled: true
    params:
      stale_threshold_days: 60   # Issues inactive longer than this are flagged
      zombie_threshold_days: 365 # Issues older than this are considered zombies

  repo_health:
    enabled: true

  ci:
    enabled: true

  security:
    enabled: true

  releases:
    enabled: true

  branches:
    enabled: true
    params:
      stale_threshold_days: 90  # Branches inactive longer than this are flagged

  dependencies:
    enabled: true
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a default configuration file",
	Long: `Creates a default configuration file (config.yaml) in your user configuration directory if it doesn't exist.
Use this to customize analysis thresholds, enable/disable specific analyzers, and set global defaults.

Note: 'gh-inspect run', 'org', etc. will automatically create this file if it's missing.
'gh-inspect init' is useful if you want to inspect or customize the config before running any analysis.`,
	Run: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// createDefaultConfig writes the default configuration to the specified path
func createDefaultConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}
	return os.WriteFile(path, []byte(defaultConfig), 0600)
}

func runInit(cmd *cobra.Command, args []string) {
	configPath, err := config.GetConfigPath()
	if err != nil {
		fmt.Printf("Error getting config path: %v\n", err)
		os.Exit(1)
	}

	// Check if file already exists to prevent overwriting
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("⚠️  Checking %s... already exists.\n", configPath)
		fmt.Println("Aborting to prevent overwrite. Delete the existing file first if you want to regenerate it.")
		return
	}

	if err := createDefaultConfig(configPath); err != nil {
		fmt.Printf("❌ Error creating config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully created %s\n", configPath)
	fmt.Println("You can now edit this file to configure thresholds and enabled analyzers.")
}
