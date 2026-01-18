package cli

import (
	"fmt"
	"os"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/mikematt33/gh-inspect/internal/report"
	"github.com/spf13/cobra"
)

// Version can be set via build flags: -ldflags "-X 'github.com/mikematt33/gh-inspect/internal/cli.Version=v1.0.0'"
var Version = "dev"

var (
	rootCmd = &cobra.Command{
		Use:   "gh-inspect",
		Short: "GitHub Repository Deep Inspection Tool",
		Long: `gh-inspect is a CLI tool for comprehensive engineering health analysis of GitHub repositories.
It measures commit patterns, PR velocity, issue hygiene, CI stability, and more to provide a holistic health score.`,
		Version:          Version,
		PersistentPreRun: checkAndInitConfig,
	}

	runCmd = &cobra.Command{
		Use:   "run [repos...]",
		Short: "Run analysis on one or more repositories (format: owner/repo)",
		Long: `Analyze one or more GitHub repositories.
This command performs a deep dive into the specified repositories, gathering metrics on activity, pull requests, issues, and CI workflows.`,
		Example: `  gh-inspect run owner/repo
  gh-inspect run owner/repo1 owner/repo2 --deep
  gh-inspect run owner/repo --format=json > report.json`,
		Args: cobra.MinimumNArgs(1),
		Run:  runAnalysis,
	}
)

// Flags
var (
	flagFormat string
	flagSince  string
	flagDeep   bool
	flagFail   int
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func checkAndInitConfig(cmd *cobra.Command, args []string) {
	// Skip for init, config, help, completion
	if cmd == initCmd || cmd == configCmd || cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "__complete" {
		return
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		// Can't resolve path, probably can't save either. Ignore.
		return
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("ℹ️  Config not found at %s. Initializing default configuration...\n", configPath)
		if err := createDefaultConfig(configPath); err != nil {
			fmt.Printf("⚠️  Failed to auto-create config: %v\n", err)
		} else {
			fmt.Println("✅ Config created.")
		}
	}
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&flagFormat, "format", "f", "text", "Output format (text, json)")
	runCmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window (e.g. 30d, 24h)")
	runCmd.Flags().BoolVarP(&flagDeep, "deep", "d", false, "Enable deep scanning (warning: consumes more API rate limit)")
	runCmd.Flags().IntVar(&flagFail, "fail-under", 0, "Exit with error code 1 if average health score is below this value")

	// Register compare command
	rootCmd.AddCommand(compareCmd)
	compareCmd.Flags().StringVarP(&flagFormat, "format", "f", "text", "Output format (text, json)")
	compareCmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window (e.g. 30d, 24h)")
	compareCmd.Flags().BoolVarP(&flagDeep, "deep", "d", false, "Enable deep scanning (warning: consumes more API rate limit)")
}

func runAnalysis(cmd *cobra.Command, args []string) {
	opts := AnalysisOptions{
		Repos: args,
		Since: flagSince,
		Deep:  flagDeep,
	}

	fullReport, err := pipelineRunner(opts)
	if err != nil {
		fmt.Printf("Error running analysis: %v\n", err)
		os.Exit(1)
	}

	// 4. Render Output
	var renderer report.Renderer
	if flagFormat == "json" {
		renderer = &report.JSONRenderer{}
	} else {
		renderer = &report.TextRenderer{}
	}

	if err := renderer.Render(fullReport, os.Stdout); err != nil {
		fmt.Printf("Error rendering report: %v\n", err)
	}

	// Exit Code Check
	if flagFail > 0 && fullReport.Summary.AvgHealthScore < float64(flagFail) {
		fmt.Printf("\n❌ Failure: Average health score (%.1f) is below threshold (%d).\n", fullReport.Summary.AvgHealthScore, flagFail)
		os.Exit(1)
	}
}
