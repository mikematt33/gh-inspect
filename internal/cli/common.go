package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/activity"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/branches"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/ci"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/issuehygiene"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/prflow"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/releases"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/repohealth"
	"github.com/mikematt33/gh-inspect/internal/analysis/analyzers/security"
	"github.com/mikematt33/gh-inspect/internal/config"
	ghclient "github.com/mikematt33/gh-inspect/internal/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
	"github.com/schollz/progressbar/v3"
)

// getClientWithToken initializes a GitHub client with token resolution and validation.
// It attempts to resolve the token from configuration, environment, or gh CLI.
// Returns an error if no valid token is found.
func getClientWithToken(cfg *config.Config) (*ghclient.ClientWrapper, error) {
	token := ghclient.ResolveToken(cfg.Global.GitHubToken)
	if token == "" {
		return nil, fmt.Errorf("no GitHub token found. Please run 'gh-inspect auth' to login")
	}
	return ghclient.NewClient(token), nil
}

// AnalysisOptions contains the configuration for running repository analysis.
type AnalysisOptions struct {
	Repos   []string
	Since   string
	Deep    bool
	Include []string
	Exclude []string
}

var pipelineRunner = RunAnalysisPipeline

// shouldIncludeAnalyzer determines if an analyzer should be included based on include/exclude filters.
// If include list is provided, only those analyzers are included.
// If exclude list is provided, all analyzers except those are included.
// Include takes precedence over exclude.
func shouldIncludeAnalyzer(analyzerName string, include, exclude []string) bool {
	// Map full analyzer names to their short names
	shortName := analyzerName
	switch analyzerName {
	case "pr-flow":
		shortName = "prflow"
	case "repo-health":
		shortName = "health"
	case "issue-hygiene":
		shortName = "issues"
	}

	// If include list is specified, only include analyzers in the list
	if len(include) > 0 {
		for _, name := range include {
			if name == shortName || name == analyzerName {
				return true
			}
		}
		return false
	}

	// If exclude list is specified, exclude analyzers in the list
	if len(exclude) > 0 {
		for _, name := range exclude {
			if name == shortName || name == analyzerName {
				return false
			}
		}
	}

	return true
}

// RunAnalysisPipeline executes the complete analysis workflow for the specified repositories.
// It loads configuration, sets up analyzers, runs analysis concurrently, and aggregates results.
// The function supports context cancellation and provides progress feedback.
func RunAnalysisPipeline(opts AnalysisOptions) (*models.Report, error) {
	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	// 2. Parse Time Window
	var duration time.Duration

	if strings.HasSuffix(opts.Since, "d") {
		daysStr := strings.TrimSuffix(opts.Since, "d")
		var days int
		_, scanErr := fmt.Sscanf(daysStr, "%d", &days)
		if scanErr != nil {
			err = scanErr
		} else {
			duration = time.Duration(days) * 24 * time.Hour
		}
	} else {
		duration, err = time.ParseDuration(opts.Since)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid time duration format: %s. Use '30d' or '720h'", opts.Since)
	}

	analysisCfg := analysis.Config{
		Since:       time.Now().Add(-duration),
		IncludeDeep: opts.Deep,
	}

	// 3. Setup Dependencies
	token := ghclient.ResolveToken(cfg.Global.GitHubToken)
	if token == "" {
		return nil, fmt.Errorf("no GitHub token found. Please run 'gh-inspect auth' to login")
	}
	client := ghclient.NewClient(token)

	// Pre-flight check for rate limits
	limits, err := client.GetRateLimit(context.Background())
	if err != nil {
		// Warning only - don't fail
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Could not check rate limit: %v\n", err)
	} else {
		// Estimate cost based on scan depth
		costPerRepo := 25 // Base estimate (commits, health, basic stats)
		if opts.Deep {
			costPerRepo = 150 // Deep scan includes issue pagination, reviews, etc.
		}

		totalCost := costPerRepo * len(opts.Repos)
		if limits.Remaining < totalCost {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Analysis may exhaust rate limit. Estimated ~%d requests needed, %d remaining.\n", totalCost, limits.Remaining)
			fmt.Fprintf(os.Stderr, "   Proceeding anyway in 2 seconds (Ctrl+C to cancel)...\n")
			time.Sleep(2 * time.Second)
		}
	}

	// Setup Analyzer Registry
	var analyzers []analysis.Analyzer

	// Always add Activity (Tier 1) if included
	if shouldIncludeAnalyzer("activity", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, activity.New())
	}

	if cfg.Analyzers.PRFlow.Enabled && shouldIncludeAnalyzer("pr-flow", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, prflow.New(cfg.Analyzers.PRFlow.Params.StaleThresholdDays))
	}

	if cfg.Analyzers.RepoHealth.Enabled && shouldIncludeAnalyzer("repo-health", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, repohealth.New())
	}

	if cfg.Analyzers.IssueHygiene.Enabled && shouldIncludeAnalyzer("issue-hygiene", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, issuehygiene.New(
			cfg.Analyzers.IssueHygiene.Params.StaleThresholdDays,
			cfg.Analyzers.IssueHygiene.Params.ZombieThresholdDays,
		))
	}

	if cfg.Analyzers.CI.Enabled && shouldIncludeAnalyzer("ci", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, ci.New())
	}

	if cfg.Analyzers.Security.Enabled && shouldIncludeAnalyzer("security", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, security.New())
	}

	if cfg.Analyzers.Releases.Enabled && shouldIncludeAnalyzer("releases", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, releases.New())
	}

	if cfg.Analyzers.Branches.Enabled && shouldIncludeAnalyzer("branches", opts.Include, opts.Exclude) {
		analyzers = append(analyzers, branches.New(cfg.Analyzers.Branches.Params.StaleThresholdDays))
	}

	start := time.Now()

	// Setup context with cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\n⚠️  Received interrupt signal. Cancelling analysis...")
		cancel()
	}()

	// Concurrency control
	maxworkers := cfg.Global.Concurrency
	if maxworkers < 1 {
		maxworkers = 1
	}
	sem := make(chan struct{}, maxworkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Track progress
	var completed int
	totalRepos := len(opts.Repos)

	// Create progress bar if not in quiet mode
	var bar *progressbar.ProgressBar
	if shouldPrintInfo() {
		bar = progressbar.NewOptions(totalRepos,
			progressbar.OptionSetDescription("Analyzing repositories"),
			progressbar.OptionSetWidth(40),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("repos"),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionClearOnFinish(),
		)
	}

	// Prepare Report Struct matching models/report.go definition
	fullReport := models.Report{
		Meta: models.ReportMeta{
			GeneratedAt: time.Now(),
			CLIVersion:  Version,
			Command:     "run", // This might need to be passed in or generic
		},
		Repositories: []models.RepoResult{},
	}

	if shouldPrintInfo() {
		fmt.Printf("Queueing %d repositories (concurrency: %d)...\n", len(opts.Repos), maxworkers)
	}

	for _, repoArg := range opts.Repos {
		wg.Add(1)
		go func(arg string) {
			defer wg.Done()

			// Check for cancellation
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			parts := strings.Split(arg, "/")
			if len(parts) != 2 {
				fmt.Printf("Skipping invalid repo format: %s\n", arg)
				return
			}

			owner, name := parts[0], parts[1]
			if shouldPrintVerbose() {
				fmt.Printf("Analyzing %s/%s...\n", owner, name)
			}

			repoReport := models.RepoResult{
				Name:      fmt.Sprintf("%s/%s", owner, name),
				URL:       fmt.Sprintf("https://github.com/%s/%s", owner, name),
				Analyzers: []models.AnalyzerResult{},
			}

			target := analysis.TargetRepository{Owner: owner, Name: name}

			for _, az := range analyzers {
				// Check for cancellation before each analyzer
				select {
				case <-ctx.Done():
					return
				default:
				}

				res, err := az.Analyze(ctx, client, target, analysisCfg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error analyzing %s with %s: %v\n", arg, az.Name(), err)
					// Add placeholder error result
					res.Name = az.Name()
					res.Findings = append(res.Findings, models.Finding{
						Type:     "analyzer_error",
						Severity: models.SeverityHigh,
						Message:  fmt.Sprintf("Analysis failed: %v", err),
					})
				}
				repoReport.Analyzers = append(repoReport.Analyzers, res)
			}

			mu.Lock()
			fullReport.Repositories = append(fullReport.Repositories, repoReport)
			completed++
			if bar != nil {
				_ = bar.Add(1)
			} else if shouldPrintVerbose() {
				fmt.Printf("✓ Completed %s/%s (%d/%d repositories)\n", owner, name, completed, totalRepos)
			}
			mu.Unlock()

		}(repoArg)
	}

	wg.Wait()

	// Finish progress bar
	if bar != nil {
		_ = bar.Finish()
	}

	// Check if analysis was cancelled
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("analysis cancelled by user")
	default:
	}

	durationScan := time.Since(start)
	fullReport.Meta.Duration = durationScan.String()

	// Calculate Global Summary in a single pass
	fullReport.Summary.TotalReposAnalyzed = len(fullReport.Repositories)

	var sumHealth, sumCISuccess, sumPRCycle float64
	var countHealth, countCI, countPRCycle int

	for _, r := range fullReport.Repositories {
		for _, az := range r.Analyzers {
			fullReport.Summary.IssuesFound += len(az.Findings)

			for _, m := range az.Metrics {
				switch m.Key {
				case "commits_total":
					fullReport.Summary.TotalCommits += int(m.Value)
				case "open_issues_total":
					fullReport.Summary.TotalOpenIssues += int(m.Value)
				case "zombie_issues":
					fullReport.Summary.TotalZombieIssues += int(m.Value)
				case "health_score":
					sumHealth += m.Value
					countHealth++
					if m.Value < 50.0 {
						fullReport.Summary.ReposAtRisk++
					}
				case "success_rate":
					sumCISuccess += m.Value
					countCI++
				case "bus_factor":
					if m.Value == 1 {
						fullReport.Summary.BusFactor1Repos++
					}
				case "avg_cycle_time_hours":
					sumPRCycle += m.Value
					countPRCycle++
				}
			}
		}
	}

	if countHealth > 0 {
		fullReport.Summary.AvgHealthScore = sumHealth / float64(countHealth)
	}
	if countCI > 0 {
		fullReport.Summary.AvgCISuccessRate = sumCISuccess / float64(countCI)
	}
	if countPRCycle > 0 {
		fullReport.Summary.AvgPRCycleTime = sumPRCycle / float64(countPRCycle)
	}

	return &fullReport, nil
}
