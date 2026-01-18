package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/config"
	ghclient "github.com/mikematt33/gh-inspect/internal/github"
	"github.com/mikematt33/gh-inspect/internal/report"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user [username]",
	Short: "Analyze all repositories of a user",
	Long: `Scan all active public repositories belonging to a specific GitHub user.
Useful for personal portfolio reviews or analyzing open source contributions.`,
	Example: `  gh-inspect user octocat
  gh-inspect user octocat --deep`,
	Args: cobra.ExactArgs(1),
	Run:  runUserAnalysis,
}

var getUserRepositories = func(username string) ([]*github.Repository, error) {
	cfg, _ := config.Load()
	token := ""
	if cfg != nil {
		token = cfg.Global.GitHubToken
	}
	finalToken := ghclient.ResolveToken(token)

	if finalToken == "" {
		return nil, fmt.Errorf("no GitHub token found. Please run 'gh-inspect auth' to login")
	}
	client := ghclient.NewClient(finalToken)

	return client.ListUserRepositories(context.Background(), username, nil)
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.Flags().StringVarP(&flagFormat, "format", "f", "text", "Output format (text, json)")
	userCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	userCmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window")
	userCmd.RegisterFlagCompletionFunc("since", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"30d", "90d", "180d", "24h", "720h"}, cobra.ShellCompDirectiveNoFileComp
	})

	userCmd.Flags().BoolVarP(&flagDeep, "deep", "d", false, "Enable deep scanning")
	userCmd.Flags().IntVar(&flagFail, "fail-under", 0, "Exit with error code 1 if average health score is below this value")
}

func runUserAnalysis(cmd *cobra.Command, args []string) {
	username := args[0]
	fmt.Printf("Fetching repositories for user '%s'...\n", username)

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

	fmt.Printf("found %d total repositories\n", len(repos))
	fmt.Printf("analyzing %d active repositories (%d archived, %d forks included)\n", len(targetRepos), archivedCount, forkCount)

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
