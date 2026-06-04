package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mikematt33/gh-inspect/internal/analysis/actions"
	"github.com/mikematt33/gh-inspect/internal/config"
	ghclient "github.com/mikematt33/gh-inspect/internal/github"
	"github.com/mikematt33/gh-inspect/internal/report"
	"github.com/mikematt33/gh-inspect/pkg/models"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// actions command flags
var (
	flagActionsRepos      []string
	flagActionsOrg        string
	flagActionsDays       int
	flagActionsWorkflow   string
	flagActionsTop        int
	flagActionsMaxRuns    int
	flagActionsSampleJobs int
	flagActionsFormat     string
	flagActionsOutputFile string
	flagActionsDryRun     bool
	flagActionsConfirm    bool

	// auth flags
	flagActionsTokens       []string
	flagActionsAppID        int64
	flagActionsAppKey       string
	flagActionsInstallation int64
)

var actionsCmd = &cobra.Command{
	Use:   "actions [owner/repo...]",
	Short: "Deep analytics on GitHub Actions across repos or an organization",
	Long: `Analyze GitHub Actions usage and health across one or more repositories or an
entire organization. Reports workflow inventory, run success rates, duration
metrics, runner usage, queue times, job breakdowns, scheduled-workflow status,
and org-level roll-ups.

Authentication supports multiple PATs (with quota-aware rotation) and GitHub App
installations (short-lived installation tokens, auto-refreshed). A pre-flight
estimator projects the API cost of large scans before they run.

Required permissions:
  PAT (classic):       repo (private) or public_repo, plus read:org for org scans;
                       workflow scope is NOT required for read-only analytics.
  PAT (fine-grained):  Actions: read, Metadata: read, Administration: read
                       (Administration is only needed for self-hosted runner detail).
  GitHub App:          actions:read, metadata:read, administration:read.`,
	Example: `  gh-inspect actions --repo owner/repo
  gh-inspect actions --repo owner/repo1 --repo owner/repo2
  gh-inspect actions --org myorg
  gh-inspect actions --repo owner/repo --days 60
  gh-inspect actions --repo owner/repo --workflow ci.yml
  gh-inspect actions --org myorg --top 10
  gh-inspect actions --org myorg --dry-run
  gh-inspect actions --org myorg --confirm
  gh-inspect actions --token ghp_aaa --token ghp_bbb --repo owner/repo
  gh-inspect actions --app-id 12345 --app-key ./key.pem --installation-id 67890 --org myorg`,
	Args: func(cmd *cobra.Command, args []string) error {
		if flagActionsFormat != "" && flagActionsFormat != "text" && flagActionsFormat != "json" && flagActionsFormat != "markdown" {
			return fmt.Errorf("invalid format: %s (must be text, json, or markdown)", flagActionsFormat)
		}
		return nil
	},
	Run: runActions,
}

func init() {
	rootCmd.AddCommand(actionsCmd)

	actionsCmd.Flags().StringArrayVar(&flagActionsRepos, "repo", nil, "Repository to scan (owner/repo); repeatable")
	actionsCmd.Flags().StringVar(&flagActionsOrg, "org", "", "Scan all repositories in an organization")
	actionsCmd.Flags().IntVar(&flagActionsDays, "days", 0, "Lookback window in days (default from config, 30)")
	actionsCmd.Flags().StringVar(&flagActionsWorkflow, "workflow", "", "Restrict analysis to a single workflow (file name or display name)")
	actionsCmd.Flags().IntVar(&flagActionsTop, "top", 0, "Top-N slowest jobs / failure hotspots to report (default 5)")
	actionsCmd.Flags().IntVar(&flagActionsMaxRuns, "max-runs", 0, "Maximum runs to fetch per repo (default from config, 1000)")
	actionsCmd.Flags().IntVar(&flagActionsSampleJobs, "sample-jobs", 0, "Recent runs per workflow to inspect for job/runner detail (0 disables)")
	actionsCmd.Flags().StringVarP(&flagActionsFormat, "format", "f", "text", "Output format (text, json, markdown)")
	actionsCmd.Flags().StringVar(&flagActionsOutputFile, "output-file", "", "Write report to file (.json/.md/.txt) in addition to stdout")
	actionsCmd.Flags().BoolVar(&flagActionsDryRun, "dry-run", false, "Run only the pre-flight estimate and exit")
	actionsCmd.Flags().BoolVar(&flagActionsConfirm, "confirm", false, "Proceed past pre-flight without prompting (for scripts); fails if quota is insufficient")

	actionsCmd.Flags().StringArrayVar(&flagActionsTokens, "token", nil, "GitHub PAT to add to the rotation pool; repeatable")
	actionsCmd.Flags().Int64Var(&flagActionsAppID, "app-id", 0, "GitHub App ID")
	actionsCmd.Flags().StringVar(&flagActionsAppKey, "app-key", "", "GitHub App private key (PEM file path or inline PEM)")
	actionsCmd.Flags().Int64Var(&flagActionsInstallation, "installation-id", 0, "GitHub App installation ID")

	_ = actionsCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json", "markdown"}, cobra.ShellCompDirectiveNoFileComp
	})
}

func runActions(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Resolve targets: positional args + --repo flags, or --org.
	repoArgs := append([]string{}, args...)
	repoArgs = append(repoArgs, flagActionsRepos...)
	if len(repoArgs) == 0 && flagActionsOrg == "" {
		fmt.Fprintln(os.Stderr, "Error: specify at least one --repo owner/repo or --org <organization>")
		os.Exit(1)
	}
	if len(repoArgs) > 0 && flagActionsOrg != "" {
		fmt.Fprintln(os.Stderr, "Error: use either --repo or --org, not both")
		os.Exit(1)
	}

	// Build credential pool from flags + config + environment.
	pool, err := buildActionsPool(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := ghclient.NewActionsClient(pool)

	if shouldPrintInfo() {
		fmt.Fprintln(os.Stderr, "Checking credential quota...")
	}
	pool.Prime(ctx)

	// Resolve the repository list.
	repos, err := resolveActionsRepos(ctx, client, repoArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving repositories: %v\n", err)
		os.Exit(1)
	}
	if len(repos) == 0 {
		fmt.Fprintln(os.Stderr, "No repositories to scan.")
		return
	}

	opts := actionsOptions(cfg, cmd)

	// Pre-flight estimate.
	avgWF := measureAvgWorkflows(ctx, client, repos)
	est := actions.EstimatePreflight(actions.PreflightInput{
		Repos:               len(repos),
		AvgWorkflowsPerRepo: avgWF,
		SampleJobRuns:       opts.SampleJobRuns,
		MaxRunsPerRepo:      opts.MaxRuns,
	}, pool)

	report.RenderPreflight(&est, os.Stderr)

	if flagActionsDryRun {
		// Emit machine-readable preflight when JSON is requested.
		if flagActionsFormat == "json" {
			out := &models.ActionsReport{
				Meta:        actionsMeta(cfg, repoArgs),
				Preflight:   &est,
				TokenStatus: pool.Summaries(),
			}
			_ = report.NewActionsRenderer("json").Render(out, os.Stdout)
		}
		return
	}

	// Confirmation gate for large scans.
	if !confirmScan(&est, cfg.Actions.ConfirmThreshold) {
		os.Exit(1)
	}

	// Run the scan.
	start := time.Now()
	repoReports := scanRepos(ctx, client, repos, opts)

	full := &models.ActionsReport{
		Meta:         actionsMeta(cfg, repoArgs),
		Repositories: repoReports,
		Summary:      actions.BuildOrgSummary(repoReports, opts.Top),
		Preflight:    &est,
		TokenStatus:  pool.Summaries(),
	}
	full.Meta.Duration = time.Since(start).String()

	// Render.
	renderer := report.NewActionsRenderer(flagActionsFormat)
	if err := renderer.Render(full, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering report: %v\n", err)
	}

	if flagActionsOutputFile != "" {
		writeActionsOutputFile(full)
	}

	// GitHub Actions step summary integration.
	if summaryPath := os.Getenv("GITHUB_STEP_SUMMARY"); summaryPath != "" && flagActionsFormat == "markdown" {
		if f, ferr := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); ferr == nil {
			defer func() { _ = f.Close() }()
			_ = report.NewActionsRenderer("markdown").Render(full, f)
		}
	}

	// Surface exhausted / invalid credentials.
	for _, ts := range full.TokenStatus {
		if ts.Invalid {
			fmt.Fprintf(os.Stderr, "⚠️  Credential %q (%s) was invalid.\n", ts.Name, ts.Kind)
		} else if ts.Exhausted {
			fmt.Fprintf(os.Stderr, "⚠️  Credential %q (%s) was exhausted during the scan.\n", ts.Name, ts.Kind)
		}
	}
}

// actionsOptions builds engine options from config defaults overridden by flags.
func actionsOptions(cfg *config.Config, cmd *cobra.Command) actions.Options {
	opts := actions.Options{
		Days:                 cfg.Actions.Days,
		MaxRuns:              cfg.Actions.MaxRuns,
		SampleJobRuns:        cfg.Actions.SampleJobRuns,
		DurationThresholdSec: cfg.Actions.DurationThresholdSec,
		QueueThresholdSec:    cfg.Actions.QueueThresholdSec,
		WorkflowFilter:       flagActionsWorkflow,
		Top:                  flagActionsTop,
	}
	if flagActionsDays > 0 {
		opts.Days = flagActionsDays
	}
	if flagActionsMaxRuns > 0 {
		opts.MaxRuns = flagActionsMaxRuns
	}
	if cmdFlagChanged(cmd, "sample-jobs") {
		opts.SampleJobRuns = flagActionsSampleJobs
	}
	return opts
}

func cmdFlagChanged(cmd *cobra.Command, name string) bool {
	f := cmd.Flags().Lookup(name)
	return f != nil && f.Changed
}

func actionsMeta(cfg *config.Config, repoArgs []string) models.ActionsMeta {
	scope := "repo"
	if flagActionsOrg != "" {
		scope = "org"
	} else if len(repoArgs) > 1 {
		scope = "multi-repo"
	}
	days := cfg.Actions.Days
	if flagActionsDays > 0 {
		days = flagActionsDays
	}
	return models.ActionsMeta{
		GeneratedAt: time.Now(),
		CLIVersion:  Version,
		Scope:       scope,
		WindowDays:  days,
	}
}

// buildActionsPool assembles the credential pool from CLI flags, config, and
// environment (PATs and GitHub App installations).
func buildActionsPool(cfg *config.Config) (*ghclient.Pool, error) {
	var sources []ghclient.TokenSource
	seen := map[string]bool{}

	addPAT := func(tok, name string) {
		tok = strings.TrimSpace(tok)
		if tok == "" || seen[tok] {
			return
		}
		seen[tok] = true
		sources = append(sources, ghclient.NewStaticTokenSource(tok, name))
	}

	// CLI tokens first.
	for i, t := range flagActionsTokens {
		addPAT(t, fmt.Sprintf("cli-token-%d", i+1))
	}
	// Config token pool + single token field.
	for i, t := range cfg.Global.GitHubTokens {
		addPAT(t, fmt.Sprintf("config-token-%d", i+1))
	}
	for i, t := range strings.Split(cfg.Global.GitHubToken, ",") {
		addPAT(t, fmt.Sprintf("config-token-pat-%d", i+1))
	}
	// Environment / gh CLI fallback only when no PAT supplied yet.
	if len(sources) == 0 {
		for i, t := range ghclient.ResolveTokens("") {
			addPAT(t, fmt.Sprintf("env-token-%d", i+1))
		}
	}

	// CLI GitHub App.
	if flagActionsAppID != 0 && flagActionsInstallation != 0 && flagActionsAppKey != "" {
		src, err := buildAppSource(config.AppConfig{
			Name:           "cli-app",
			AppID:          flagActionsAppID,
			InstallationID: flagActionsInstallation,
			PrivateKey:     flagActionsAppKey,
		})
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}

	// Config GitHub Apps.
	for _, app := range cfg.Global.Apps {
		src, err := buildAppSource(app)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no GitHub credentials found. Provide --token, --app-id/--app-key/--installation-id, configure github_tokens/github_apps, or run 'gh-inspect auth'")
	}
	return ghclient.NewPool(sources...), nil
}

func buildAppSource(app config.AppConfig) (ghclient.TokenSource, error) {
	keyRef := app.PrivateKey
	if keyRef == "" {
		keyRef = app.PrivateKeyPath
	}
	if app.AppID == 0 || app.InstallationID == 0 || keyRef == "" {
		return nil, fmt.Errorf("incomplete GitHub App config (need app_id, installation_id, and a private key)")
	}
	pem, err := ghclient.LoadAppPrivateKey(keyRef)
	if err != nil {
		return nil, err
	}
	return ghclient.NewAppTokenSource(ghclient.AppCredentials{
		Name:           app.Name,
		AppID:          app.AppID,
		InstallationID: app.InstallationID,
		PrivateKeyPEM:  pem,
	})
}

// resolveActionsRepos expands the target list, either validating owner/repo
// arguments or enumerating an organization's repositories.
func resolveActionsRepos(ctx context.Context, client *ghclient.ActionsClient, repoArgs []string) ([][2]string, error) {
	if flagActionsOrg != "" {
		if shouldPrintInfo() {
			fmt.Fprintf(os.Stderr, "Fetching repositories for organization '%s'...\n", flagActionsOrg)
		}
		repos, err := client.ListOrgRepositories(ctx, flagActionsOrg)
		if err != nil {
			return nil, err
		}
		filter, ferr := NewRepoFilter()
		if ferr != nil {
			return nil, ferr
		}
		names, _ := FilterRepositories(repos, filter)
		out := make([][2]string, 0, len(names))
		for _, full := range names {
			if o, r, ok := splitRepo(full); ok {
				out = append(out, [2]string{o, r})
			}
		}
		return out, nil
	}

	out := make([][2]string, 0, len(repoArgs))
	for _, arg := range repoArgs {
		o, r, ok := splitRepo(arg)
		if !ok {
			return nil, fmt.Errorf("invalid repository %q (expected owner/repo)", arg)
		}
		recordUsage(arg, "repo")
		out = append(out, [2]string{o, r})
	}
	return out, nil
}

func splitRepo(full string) (string, string, bool) {
	parts := strings.Split(full, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// measureAvgWorkflows samples up to three repositories to refine the workflow
// count estimate used for pre-flight. Sampling failures fall back to a default.
func measureAvgWorkflows(ctx context.Context, client *ghclient.ActionsClient, repos [][2]string) float64 {
	sample := repos
	if len(sample) > 3 {
		sample = sample[:3]
	}
	total := 0
	measured := 0
	for _, rp := range sample {
		wfs, err := client.ListWorkflows(ctx, rp[0], rp[1])
		if err != nil {
			continue
		}
		total += len(wfs)
		measured++
	}
	if measured == 0 {
		return 5
	}
	return float64(total) / float64(measured)
}

// confirmScan enforces the confirmation policy for large or under-quota scans.
// Returns true if the scan may proceed.
func confirmScan(est *models.PreflightEstimate, threshold int) bool {
	large := threshold > 0 && est.EstimatedAPICalls > threshold
	needsConfirm := large || !est.Sufficient

	if !needsConfirm {
		return true
	}
	if !est.Sufficient && flagActionsConfirm {
		// Explicit confirmation cannot override insufficient quota in scripts.
		fmt.Fprintln(os.Stderr, "✗ Insufficient quota across all credentials to complete this scan.")
		return false
	}
	if flagActionsConfirm {
		return true
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "Large scan (~%d API calls > threshold %d). Re-run with --confirm to proceed.\n", est.EstimatedAPICalls, threshold)
		return false
	}
	fmt.Fprintf(os.Stderr, "Proceed with scan of ~%d API calls? [y/N] ", est.EstimatedAPICalls)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// scanRepos analyzes each repository, with bounded concurrency and a progress
// bar when appropriate.
func scanRepos(ctx context.Context, client *ghclient.ActionsClient, repos [][2]string, opts actions.Options) []models.ActionsRepoReport {
	engine := actions.NewEngine(client)

	cfg, _ := config.Load()
	workers := 1
	if cfg != nil && cfg.Global.Concurrency > 1 {
		workers = cfg.Global.Concurrency
	}
	if workers > len(repos) {
		workers = len(repos)
	}
	if workers < 1 {
		workers = 1
	}

	var bar *progressbar.ProgressBar
	if shouldPrintInfo() && !shouldPrintVerbose() {
		bar = progressbar.NewOptions(len(repos),
			progressbar.OptionSetDescription("Scanning Actions"),
			progressbar.OptionSetWidth(40),
			progressbar.OptionShowCount(),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionClearOnFinish(),
		)
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]models.ActionsRepoReport, 0, len(repos))

	for _, rp := range repos {
		wg.Add(1)
		go func(owner, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if shouldPrintVerbose() {
				fmt.Fprintf(os.Stderr, "Scanning %s/%s...\n", owner, name)
			}
			rr, err := engine.ScanRepo(ctx, owner, name, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scanning %s/%s: %v\n", owner, name, err)
			}
			mu.Lock()
			results = append(results, rr)
			mu.Unlock()
			if bar != nil {
				_ = bar.Add(1)
			}
		}(rp[0], rp[1])
	}
	wg.Wait()
	if bar != nil {
		_ = bar.Finish()
	}

	// Stable ordering by repo name for deterministic output.
	sortRepoReports(results)
	return results
}

func sortRepoReports(reports []models.ActionsRepoReport) {
	for i := 1; i < len(reports); i++ {
		for j := i; j > 0 && reports[j].Name < reports[j-1].Name; j-- {
			reports[j], reports[j-1] = reports[j-1], reports[j]
		}
	}
}

func writeActionsOutputFile(full *models.ActionsReport) {
	format := "text"
	switch {
	case strings.HasSuffix(flagActionsOutputFile, ".json"):
		format = "json"
	case strings.HasSuffix(flagActionsOutputFile, ".md"):
		format = "markdown"
	}
	f, err := os.Create(flagActionsOutputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }()
	if err := report.NewActionsRenderer(format).Render(full, f); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
	} else if shouldPrintInfo() {
		fmt.Fprintf(os.Stderr, "\n✅ Report written to %s\n", flagActionsOutputFile)
	}
}
