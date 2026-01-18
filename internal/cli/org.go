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

var getOrgRepositories = func(orgName string) ([]*github.Repository, error) {
	// 1. Setup Client to list repos
	// Note: We don't have access to full config here effortlessly without loading it again.
	// For CLI simplicity, we'll try ResolveToken with empty string first (env/gh cli)
	// If that fails, we might miss the config token.
	// ideally, org command should load config first.
	// Refactoring org command to load config is better, but let's see.
	// Actually, runOrgAnalysis calls RunAnalysisPipeline later, but we need the client NOW to list repos.

	// Let's load config just for the token
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

	return client.ListRepositories(context.Background(), orgName, nil)
}

var orgCmd = &cobra.Command{
	Use:   "org [organization]",
	Short: "Analyze an entire GitHub organization",
	Long: `Scan all active repositories in a GitHub organization.
Automatically fetches all repositories, filters out archived ones, and runs the health analysis on each.`,
	Example: `  gh-inspect org my-org
  gh-inspect org my-org --fail-under=80`,
	Args: cobra.ExactArgs(1),
	Run:  runOrgAnalysis,
}

func init() {
	rootCmd.AddCommand(orgCmd)
	orgCmd.Flags().StringVarP(&flagFormat, "format", "f", "text", "Output format (text, json)")
	orgCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	orgCmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window")
	orgCmd.RegisterFlagCompletionFunc("since", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"30d", "90d", "180d", "24h", "720h"}, cobra.ShellCompDirectiveNoFileComp
	})
	orgCmd.Flags().BoolVarP(&flagDeep, "deep", "d", false, "Enable deep scanning")
	orgCmd.Flags().IntVar(&flagFail, "fail-under", 0, "Exit with error code 1 if average health score is below this value")
}

func runOrgAnalysis(cmd *cobra.Command, args []string) {
	orgName := args[0]

	fmt.Printf("Fetching repositories for organization '%s'...\n", orgName)

	// 2. Fetch all repos
	// We pass nil options to trigger auto-pagination in our client wrapper
	repos, err := getOrgRepositories(orgName)
	if err != nil {
		fmt.Printf("Error listing repositories: %v\n", err)
		os.Exit(1)
	}

	// 3. Filter and Prepare
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

	// 4. Run Pipeline
	opts := AnalysisOptions{
		Repos: targetRepos,
		Since: flagSince, // Flag from root/org command share the same vars if defined in root?
		// checks root.go... yes, var flagFormat, flagSince, flagDeep are package variables.
		Deep: flagDeep,
	}

	fullReport, err := pipelineRunner(opts)
	if err != nil {
		fmt.Printf("Error running analysis: %v\n", err)
		os.Exit(1)
	}

	// Inject Org-level Stats into Summary (Manual Override)
	// Currently Report.Summary is rudimentary, but we can set TotalReposAnalyzed at least.
	fullReport.Summary.TotalReposAnalyzed = len(targetRepos)

	// 5. Render Output
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
