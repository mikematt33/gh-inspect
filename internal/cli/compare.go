package cli

import (
	"fmt"
	"os"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/mikematt33/gh-inspect/internal/report"
	"github.com/spf13/cobra"
)

var compareCmd = &cobra.Command{
	Use:   "compare [repos...]",
	Short: "Compare multiple repositories side-by-side",
	Long: `Run analysis on multiple repositories and display metrics in a comparison table.
Useful for benchmarking internal projects against each other or comparing against open source standards.

Minimum 2 repositories required. Supports --quiet and --verbose flags.`,
	Example: `  gh-inspect compare owner/repo1 owner/repo2
  gh-inspect compare owner/repo1 owner/repo2 owner/repo3
  gh-inspect compare owner/repo1 owner/repo2 --format=json`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Validate format
		if flagFormat != "" && flagFormat != "text" && flagFormat != "json" && flagFormat != "markdown" {
			return fmt.Errorf("invalid format: %s (must be text, json, or markdown)", flagFormat)
		}

		if flagListAnalyzers {
			return nil // Allow no args when listing analyzers
		}
		return cobra.MinimumNArgs(2)(cmd, args)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if flagListAnalyzers {
			listAnalyzers()
		}
		return nil
	},
	Run: runComparison,
}

func runComparison(cmd *cobra.Command, args []string) {
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

	fullReport.Meta.Command = "compare"

	// Render Output
	var renderer report.Renderer
	if flagFormat == "json" {
		renderer = &report.JSONRenderer{}
	} else {
		renderer = &report.ComparisonTextRenderer{}
	}

	if err := renderer.Render(fullReport, os.Stdout); err != nil {
		fmt.Printf("Error rendering report: %v\n", err)
	}
}
