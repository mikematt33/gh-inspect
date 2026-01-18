package issuehygiene

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct {
	staleThreshold  time.Duration
	zombieThreshold time.Duration
}

func New(staleDays, zombieDays int) *Analyzer {
	return &Analyzer{
		staleThreshold:  time.Duration(staleDays) * 24 * time.Hour,
		zombieThreshold: time.Duration(zombieDays) * 24 * time.Hour,
	}
}

func (a *Analyzer) Name() string {
	return "issue-hygiene"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	// 1. Fetch Open Issues (Oldest Updated first, to find stale/zombie)
	// We want to verify if there are abandoned issues.
	// We'll limit to 100 to avoid rate limits, unless Deep scan is enabled.
	openOpts := &github.IssueListByRepoOptions{
		State:       "open",
		Sort:        "updated",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	// If NOT deep scan, maybe just fetch one page?
	// The client implementation auto-paginates... which is good for full analysis but bad for quick scan.
	// But let's assume auto-pagination is fine for now; 100 per page, limited activity on many repos.

	openIssues, err := client.GetIssues(ctx, repo.Owner, repo.Name, openOpts)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}

	// 2. Fetch Recently Closed Issues (for throughput/lifetime)
	// We only care about those closed in the lookback window.
	closedOpts := &github.IssueListByRepoOptions{
		State:       "closed",
		Since:       cfg.Since,
		ListOptions: github.ListOptions{PerPage: 100},
	}
	closedIssues, err := client.GetIssues(ctx, repo.Owner, repo.Name, closedOpts)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}

	// 3. Calculate Metrics
	var staleCount int
	var zombieCount int
	var findings []models.Finding

	now := time.Now()

	for _, issue := range openIssues {
		updatedAt := issue.GetUpdatedAt()
		createdAt := issue.GetCreatedAt()

		// Stale check
		if now.Sub(updatedAt.Time) > a.staleThreshold {
			staleCount++
			// Finding for the oldest few
			if staleCount <= 3 {
				findings = append(findings, models.Finding{
					Type:        "stale_issue",
					Severity:    models.SeverityMedium,
					Message:     fmt.Sprintf("Issue #%d has been inactive for %d days", issue.GetNumber(), int(now.Sub(updatedAt.Time).Hours()/24)),
					Location:    issue.GetHTMLURL(),
					Actionable:  true,
					Remediation: "Close or Ping assignee",
				})
			}
		}

		// Zombie check (Created long ago, still open)
		if now.Sub(createdAt.Time) > a.zombieThreshold {
			zombieCount++
			if zombieCount <= 3 {
				findings = append(findings, models.Finding{
					Type:     "zombie_issue",
					Severity: models.SeverityLow,
					Message:  fmt.Sprintf("Issue #%d is a zombie (open > %d days)", issue.GetNumber(), int(now.Sub(createdAt.Time).Hours()/24)),
					Location: issue.GetHTMLURL(),
				})
			}
		}
	}

	// Lifetime calculation
	var totalLifetime time.Duration
	for _, issue := range closedIssues {
		if issue.GetClosedAt().IsZero() {
			continue
		}
		lifetime := issue.GetClosedAt().Time.Sub(issue.GetCreatedAt().Time)
		totalLifetime += lifetime
	}

	avgLifetimeHours := 0.0
	if len(closedIssues) > 0 {
		avgLifetimeHours = totalLifetime.Hours() / float64(len(closedIssues))
	}

	labeledCount := 0
	for _, issue := range openIssues {
		if len(issue.Labels) > 0 {
			labeledCount++
		}
	}
	labeledRatio := 0.0
	if len(openIssues) > 0 {
		labeledRatio = float64(labeledCount) / float64(len(openIssues))
	}

	metrics := []models.Metric{
		{Key: "open_issues_total", Value: float64(len(openIssues)), DisplayValue: fmt.Sprintf("%d", len(openIssues))},
		{Key: "closed_issues_in_window", Value: float64(len(closedIssues)), DisplayValue: fmt.Sprintf("%d", len(closedIssues))},
		{Key: "stale_issues", Value: float64(staleCount), DisplayValue: fmt.Sprintf("%d", staleCount)},
		{Key: "zombie_issues", Value: float64(zombieCount), DisplayValue: fmt.Sprintf("%d", zombieCount)},
		{Key: "avg_issue_lifetime", Value: avgLifetimeHours, Unit: "hours", DisplayValue: fmt.Sprintf("%.1fh", avgLifetimeHours)},
		{Key: "label_coverage", Value: labeledRatio, Unit: "percent", DisplayValue: fmt.Sprintf("%.0f%%", labeledRatio*100)},
	}

	if len(findings) > 0 {
		sort.Slice(findings, func(i, j int) bool {
			// sort by severity?
			return findings[i].Severity == models.SeverityHigh // simple float up
		})
	}

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
