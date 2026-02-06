package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/mikematt33/gh-inspect/internal/report"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user [username]",
	Short: "Analyze all repositories of a user",
	Long: `Scan all active public repositories belonging to a specific GitHub user.
Useful for personal portfolio reviews or analyzing open source contributions.

Displays a progress bar during analysis. Use --quiet for CI/CD environments.`,
	Example: `  gh-inspect user octocat
  gh-inspect user octocat --deep
  gh-inspect user octocat --quiet --format=json
  gh-inspect user octocat --include=activity,prflow,ci
  gh-inspect user octocat --filter-language=javascript
  gh-inspect user octocat --filter-skip-forks --filter-updated=180d`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Validate format
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
		return cobra.ExactArgs(1)(cmd, args)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if flagListAnalyzers {
			listAnalyzers()
		}
		return nil
	},
	ValidArgsFunction: completeUsers,
	Run:               runUserAnalysis,
}

var getUserRepositories = func(username string) ([]*github.Repository, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	client, err := getClientWithToken(cfg)
	if err != nil {
		return nil, err
	}

	return client.ListUserRepositories(context.Background(), username, nil)
}

func init() {
	rootCmd.AddCommand(userCmd)
	registerAnalysisFlags(userCmd)
	registerFilterFlags(userCmd)
}

func runUserAnalysis(cmd *cobra.Command, args []string) {
	username := args[0]

	// Record user usage for completions
	recordUsage(username, "user")

	if shouldPrintInfo() {
		fmt.Printf("Fetching repositories for user '%s'...\n", username)
	}

	repos, err := getUserRepositories(username)
	if err != nil {
		fmt.Printf("Error listing repositories: %v\n", err)
		os.Exit(1)
	}

	// Apply Filters
	filter, err := NewRepoFilter()
	if err != nil {
		fmt.Printf("Error creating filter: %v\n", err)
		os.Exit(1)
	}

	targetRepos, stats := FilterRepositories(repos, filter)

	if shouldPrintInfo() {
		fmt.Printf("found %d total repositories\n", stats.Total)
		if stats.Archived > 0 {
			fmt.Printf("  %d archived (skipped)\n", stats.Archived)
		}
		if stats.Forks > 0 && !flagFilterSkipForks {
			fmt.Printf("  %d forks (included)\n", stats.Forks)
		} else if flagFilterSkipForks {
			fmt.Printf("  %d forks (filtered)\n", stats.Forks)
		}
		if stats.NameFiltered > 0 {
			fmt.Printf("  %d filtered by name pattern\n", stats.NameFiltered)
		}
		if stats.LangFiltered > 0 {
			fmt.Printf("  %d filtered by language\n", stats.LangFiltered)
		}
		if stats.TopicFiltered > 0 {
			fmt.Printf("  %d filtered by topics\n", stats.TopicFiltered)
		}
		if stats.DateFiltered > 0 {
			fmt.Printf("  %d filtered by update date\n", stats.DateFiltered)
		}
		fmt.Printf("analyzing %d repositories\n", stats.Passed)
	}

	if len(targetRepos) == 0 {
		fmt.Println("No active repositories found to analyze.")
		return
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
		resolvedOutputMode = flagOutputMode
	} else if cfg.Global.OutputMode != "" {
		resolvedOutputMode = cfg.Global.OutputMode
	}

	opts := AnalysisOptions{
		Repos:           targetRepos,
		Since:           flagSince, // Uses flags from root (or init above)
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

	fullReport.Summary.TotalReposAnalyzed = len(targetRepos)

	var renderer report.Renderer
	if flagFormat == "json" {
		renderer = &report.JSONRenderer{}
	} else {
		renderer = &report.TextRenderer{}
	}

	if err := renderer.Render(fullReport, os.Stdout); err != nil {
		fmt.Printf("Error rendering report: %v\n", err)
	}

	if flagFail > 0 && fullReport.Summary.AvgHealthScore < float64(flagFail) {
		fmt.Printf("\n❌ Failure: Average health score (%.1f) is below threshold (%d).\n", fullReport.Summary.AvgHealthScore, flagFail)
		os.Exit(1)
	}
}
