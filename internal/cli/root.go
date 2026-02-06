package cli

import (
	"fmt"
	"os"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/mikematt33/gh-inspect/internal/report"
	"github.com/mikematt33/gh-inspect/pkg/baseline"
	"github.com/mikematt33/gh-inspect/pkg/models"
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
  gh-inspect run owner/repo1 owner/repo2 --depth=deep
  gh-inspect run owner/repo --format=json > report.json
  gh-inspect run owner/repo --format=markdown --explain
  gh-inspect run owner/repo --quiet --fail-under=80
  gh-inspect run owner/repo --no-cache
  gh-inspect run owner/repo --include=activity,ci,security
  gh-inspect run owner/repo --exclude=branches,releases
  gh-inspect run owner/repo --depth=shallow --max-prs=25
  gh-inspect run owner/repo --depth=standard --max-workflow-runs=200`,
		Args: func(cmd *cobra.Command, args []string) error { // Validate format
			if flagFormat != "" && flagFormat != "text" && flagFormat != "json" && flagFormat != "markdown" {
				return fmt.Errorf("invalid format: %s (must be text, json, or markdown)", flagFormat)
			}

			// Validate depth
			if flagDepth != "" && flagDepth != "shallow" && flagDepth != "standard" && flagDepth != "deep" {
				return fmt.Errorf("invalid depth: %s (must be shallow, standard, or deep)", flagDepth)
			}

			// Validate output mode
			if flagOutputMode != "" && flagOutputMode != "suggestive" && flagOutputMode != "observational" && flagOutputMode != "statistical" {
				return fmt.Errorf("invalid output mode: %s (must be suggestive, observational, or statistical)", flagOutputMode)
			}

			if flagListAnalyzers {
				return nil // Allow no args when listing analyzers
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		ValidArgsFunction: completeRepositories,
		Run: func(cmd *cobra.Command, args []string) {
			if flagListAnalyzers {
				listAnalyzers()
				return
			}
			runAnalysis(cmd, args)
		},
	}
)

// Flags
var (
	flagFormat           string
	flagSince            string
	flagDepth            string
	flagMaxPRs           int
	flagMaxIssues        int
	flagMaxWorkflowRuns  int
	flagFail             int
	flagQuiet            bool
	flagVerbose          bool
	flagInclude          []string
	flagExclude          []string
	flagListAnalyzers    bool
	flagCompareLast      bool
	flagFailOnRegression bool
	flagBaseline         string
	flagSaveBaseline     bool
	flagExplain          bool
	flagNoCache          bool
	flagOutputMode       string
	// Filtering flags
	flagFilterName      string
	flagFilterLanguage  []string
	flagFilterTopics    []string
	flagFilterUpdated   string
	flagFilterSkipForks bool
)

// listAnalyzers prints all available analyzers with descriptions
func listAnalyzers() {
	fmt.Println("Available Analyzers:")
	fmt.Println()
	fmt.Printf("  %-13s %s\n", "activity", "Commit patterns, contributors, bus factor, and code quality metrics")
	fmt.Printf("  %-13s %s\n", "prflow", "Pull request velocity, cycle time, review, and collaboration metrics")
	fmt.Printf("  %-13s %s\n", "ci", "CI/CD workflow success rates and stability")
	fmt.Printf("  %-13s %s\n", "issues", "Issue hygiene, stale issues, and zombie detection")
	fmt.Printf("  %-13s %s\n", "security", "Security advisories and vulnerability scanning")
	fmt.Printf("  %-13s %s\n", "releases", "Release frequency, deployment metrics, and versioning patterns")
	fmt.Printf("  %-13s %s\n", "branches", "Branch protection and stale branch detection")
	fmt.Printf("  %-13s %s\n", "dependencies", "Dependency management and package analysis")
	fmt.Printf("  %-13s %s\n", "health", "Repository health files (README, LICENSE, CONTRIBUTING, etc.)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  --include=activity,ci      Run only specified analyzers")
	fmt.Println("  --exclude=releases,security  Skip specified analyzers")
	fmt.Println()
	fmt.Println("Note: Analyzers can also be enabled/disabled in the config file.")
}

// registerAnalysisFlags adds common analysis flags to a command
func registerAnalysisFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&flagFormat, "format", "f", "text", "Output format (text, json, markdown)")
	_ = cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json", "markdown"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window (e.g. 30d, 24h)")
	_ = cmd.RegisterFlagCompletionFunc("since", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"30d", "90d", "180d", "24h", "720h"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().StringVar(&flagDepth, "depth", "standard", "Analysis depth: shallow, standard, or deep")
	_ = cmd.RegisterFlagCompletionFunc("depth", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"shallow", "standard", "deep"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().IntVar(&flagMaxPRs, "max-prs", 0, "Maximum PRs to analyze (0 = use depth default)")
	cmd.Flags().IntVar(&flagMaxIssues, "max-issues", 0, "Maximum issues to fetch (0 = use depth default)")
	cmd.Flags().IntVar(&flagMaxWorkflowRuns, "max-workflow-runs", 0, "Maximum CI runs to analyze (0 = use depth default)")

	cmd.Flags().IntVar(&flagFail, "fail-under", 0, "Exit with error code 1 if average health score is below this value")

	cmd.Flags().StringSliceVar(&flagInclude, "include", nil, "Only run specified analyzers (comma-separated: activity,prflow,ci,issues,security,releases,branches,dependencies,health)")
	_ = cmd.RegisterFlagCompletionFunc("include", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"activity", "prflow", "ci", "issues", "security", "releases", "branches", "dependencies", "health"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Exclude specified analyzers (comma-separated: activity,prflow,ci,issues,security,releases,branches,dependencies,health)")
	_ = cmd.RegisterFlagCompletionFunc("exclude", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"activity", "prflow", "ci", "issues", "security", "releases", "branches", "dependencies", "health"}, cobra.ShellCompDirectiveNoFileComp
	})

	cmd.Flags().BoolVar(&flagListAnalyzers, "list-analyzers", false, "List all available analyzers and exit")

	// Baseline/Comparison flags
	cmd.Flags().BoolVar(&flagCompareLast, "compare-last", false, "Compare with last saved baseline")
	cmd.Flags().StringVar(&flagBaseline, "baseline", "", "Path to baseline file to compare against")
	cmd.Flags().BoolVar(&flagSaveBaseline, "save-baseline", false, "Save this run as the new baseline")
	cmd.Flags().BoolVar(&flagFailOnRegression, "fail-on-regression", false, "Exit with error if regression detected")

	// Scoring transparency
	cmd.Flags().BoolVar(&flagExplain, "explain", false, "Show detailed score breakdown and improvement tips")

	// Output mode (how findings are presented)
	cmd.Flags().StringVar(&flagOutputMode, "output-mode", "observational", "Output mode: suggestive (prescriptive advice), observational (neutral facts, default), statistical (numbers only)")
	_ = cmd.RegisterFlagCompletionFunc("output-mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"suggestive", "observational", "statistical"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Caching
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Disable API response caching (forces fresh API calls)")
}

// registerFilterFlags adds repository filtering flags (for org and user commands)
func registerFilterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flagFilterName, "filter-name", "", "Filter repositories by name (regex pattern)")
	cmd.Flags().StringSliceVar(&flagFilterLanguage, "filter-language", nil, "Filter by primary language (comma-separated: go,python,javascript)")
	cmd.Flags().StringSliceVar(&flagFilterTopics, "filter-topics", nil, "Filter by topics/tags (comma-separated)")
	cmd.Flags().StringVar(&flagFilterUpdated, "filter-updated", "", "Filter by last update (e.g., 30d, 90d, 180d)")
	cmd.Flags().BoolVar(&flagFilterSkipForks, "filter-skip-forks", false, "Skip forked repositories")
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
	rootCmd.AddCommand(compareCmd)
	registerAnalysisFlags(runCmd)
}

func runAnalysis(cmd *cobra.Command, args []string) {
	// Record repository usage for completions
	for _, repo := range args {
		recordUsage(repo, "repo")
	}

	// Load config to get output mode preference
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Resolve output mode: flag overrides config, config overrides default
	resolvedOutputMode := "observational" // default
	if flagOutputMode != "" {
		// Flag explicitly set - use it (override config)
		resolvedOutputMode = flagOutputMode
	} else if cfg.Global.OutputMode != "" {
		// Config has a value - use it
		resolvedOutputMode = cfg.Global.OutputMode
	}

	opts := AnalysisOptions{
		Repos:           args,
		Since:           flagSince,
		Depth:           flagDepth,
		MaxPRs:          flagMaxPRs,
		MaxIssues:       flagMaxIssues,
		MaxWorkflowRuns: flagMaxWorkflowRuns,
		Include:         flagInclude,
		Exclude:         flagExclude,
		OutputMode:      resolvedOutputMode,
	}

	fullReport, err := pipelineRunner(opts)
	if err != nil {
		fmt.Printf("Error running analysis: %v\n", err)
		os.Exit(1)
	}

	// Handle baseline comparison if requested
	var comparison *baseline.ComparisonResult
	if flagCompareLast || flagBaseline != "" {
		baselinePath := flagBaseline
		if baselinePath == "" {
			baselinePath = baseline.GetDefaultBaselinePath()
		}

		previousBaseline, err := baseline.Load(baselinePath)
		if err != nil {
			if shouldPrintInfo() {
				fmt.Printf("⚠️  Could not load baseline for comparison: %v\n", err)
			}
		} else {
			comparison = baseline.Compare(fullReport, previousBaseline)
			if shouldPrintInfo() {
				printComparison(comparison)
			}

			if flagFailOnRegression && comparison != nil && comparison.Summary.HasRegression {
				fmt.Printf("\n❌ Failure: Regression detected compared to baseline.\n")
				os.Exit(1)
			}
		}
	}

	// Save baseline if requested
	if flagSaveBaseline {
		baselinePath := baseline.GetDefaultBaselinePath()
		if err := baseline.Save(fullReport, baselinePath); err != nil {
			fmt.Printf("⚠️  Failed to save baseline: %v\n", err)
		} else if shouldPrintInfo() {
			fmt.Printf("\n✅ Baseline saved to %s\n", baselinePath)
		}
	}

	// 4. Render Output
	var renderer report.Renderer
	switch flagFormat {
	case "json":
		renderer = &report.JSONRenderer{}
	case "markdown":
		renderer = &report.MarkdownRenderer{}
	default:
		renderer = &report.TextRenderer{}
	}

	// Parse output mode from the already-resolved value (respects flag > config > default)
	outputMode := models.OutputModeObservational // default
	switch resolvedOutputMode {
	case "suggestive":
		outputMode = models.OutputModeSuggestive
	case "observational", "":
		outputMode = models.OutputModeObservational
	case "statistical":
		outputMode = models.OutputModeStatistical
	}

	renderOpts := report.RenderOptions{
		ShowExplanation: flagExplain,
		OutputMode:      outputMode,
	}

	if err := renderer.RenderWithOptions(fullReport, os.Stdout, renderOpts); err != nil {
		fmt.Printf("Error rendering report: %v\n", err)
	}

	// Write to GitHub Actions Step Summary if running in GitHub Actions
	if githubStepSummary := os.Getenv("GITHUB_STEP_SUMMARY"); githubStepSummary != "" && flagFormat == "markdown" {
		f, err := os.OpenFile(githubStepSummary, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer func() { _ = f.Close() }()
			_ = renderer.RenderWithOptions(fullReport, f, renderOpts)
			if shouldPrintInfo() {
				fmt.Println("\n✅ Results written to GitHub Actions step summary")
			}
		}
	}

	// Exit Code Check for health score
	if flagFail > 0 && fullReport.Summary.AvgHealthScore < float64(flagFail) {

		fmt.Printf("\n❌ Failure: Health score is below the --fail-under threshold.\n")
		os.Exit(1)
	}
}
