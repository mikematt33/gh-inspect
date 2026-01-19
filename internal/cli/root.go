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
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	runCmd = &cobra.Command{
		Use:   "run [repos...]",
		Short: "Run analysis on one or more repositories (format: owner/repo)",
		Long: `Analyze one or more GitHub repositories with concurrent execution.
This command performs a deep dive into the specified repositories, gathering metrics on activity, pull requests, issues, and CI workflows.

The analysis runs concurrently for better performance and displays a progress bar.
Use --quiet to suppress progress output or --verbose for detailed information.`,
		Example: `  gh-inspect run owner/repo
  gh-inspect run owner/repo1 owner/repo2 --deep
  gh-inspect run owner/repo --format=json > report.json
  gh-inspect run owner/repo --quiet --fail-under=80`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeRepositories,
		Run:               runAnalysis,
	}
)

// Flags
var (
	flagFormat  string
	flagSince   string
	flagDeep    bool
	flagFail    int
	flagQuiet   bool
	flagVerbose bool
)

// registerAnalysisFlags adds common analysis flags to a command
func registerAnalysisFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagFormat, "format", "f", "text", "Output format (text, json)")
	_ = cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window (e.g. 30d, 24h)")
	_ = cmd.RegisterFlagCompletionFunc("since", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"30d", "90d", "180d", "24h", "720h"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().BoolVarP(&flagDeep, "deep", "d", false, "Enable deep scanning (warning: consumes more API rate limit)")
	cmd.Flags().IntVar(&flagFail, "fail-under", 0, "Exit with error code 1 if average health score is below this value")
}

// shouldPrintInfo returns true if informational messages should be printed (not in quiet mode)
func shouldPrintInfo() bool {
	return !flagQuiet
}

// shouldPrintVerbose returns true if verbose messages should be printed
func shouldPrintVerbose() bool {
	return flagVerbose && !flagQuiet
}

// Execute runs the root command and handles CLI execution.
// This is the main entry point for the gh-inspect CLI application.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func checkAndInitConfig(cmd *cobra.Command, args []string) {
	// Skip for init, config, help, completion, and the new auth command
	if cmd == initCmd || cmd == configCmd || cmd == authCmd || cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "__complete" {
		return
	}

	configPath, err := config.GetConfigPath()
	if err != nil {
		// Can't resolve path, probably can't save either. Ignore.
		return
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Auto-initialize default config when missing for commands other than auth/init/config.
		if shouldPrintInfo() {
			fmt.Printf("ℹ️  Config not found at %s. Initializing default configuration...\n", configPath)
		}
		if err := createDefaultConfig(configPath); err != nil {
			if shouldPrintInfo() {
				fmt.Printf("⚠️  Failed to auto-create config: %v\n", err)
			}
		} else {
			if shouldPrintInfo() {
				fmt.Println("✅ Config created.")
			}
		}
	}
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(runCmd)
	registerAnalysisFlags(runCmd)

	// Register compare command
	rootCmd.AddCommand(compareCmd)
	registerAnalysisFlags(compareCmd)
}

func runAnalysis(cmd *cobra.Command, args []string) {
	// Record repository usage for completions
	for _, repo := range args {
		recordUsage(repo, "repo")
	}

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
