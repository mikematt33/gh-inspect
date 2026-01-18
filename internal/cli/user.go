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
  gh-inspect user octocat --quiet --format=json`,
	Args: cobra.ExactArgs(1),
	Run:  runUserAnalysis,
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
}

func runUserAnalysis(cmd *cobra.Command, args []string) {
	username := args[0]
	if shouldPrintInfo() {
		fmt.Printf("Fetching repositories for user '%s'...\n", username)
	}

	repos, err := getUserRepositories(username)
	if err != nil {
		fmt.Printf("Error listing repositories: %v\n", err)
		os.Exit(1)
	}

	var targetRepos []string
	var archivedCount int
	var forkCount int

	for _, r := range repos {
		if r.GetArchived() {
			archivedCount++
			continue
		}
		if r.GetFork() {
			forkCount++
		}
		targetRepos = append(targetRepos, r.GetFullName())
	}

	if shouldPrintInfo() {
		fmt.Printf("found %d total repositories\n", len(repos))
		fmt.Printf("analyzing %d active repositories (%d archived, %d forks included)\n", len(targetRepos), archivedCount, forkCount)
	}

	if len(targetRepos) == 0 {
		fmt.Println("No active repositories found to analyze.")
		return
	}

	opts := AnalysisOptions{
		Repos: targetRepos,
		Since: flagSince, // Uses flags from root (or init above)
		Deep:  flagDeep,
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
