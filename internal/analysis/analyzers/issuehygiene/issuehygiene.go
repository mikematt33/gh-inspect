package issuehygiene

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	var assignedCount int
	var bugCount int
	var featureCount int
	var totalResponseTime time.Duration
	var responseCount int

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

		// Assignee coverage
		if len(issue.Assignees) > 0 {
			assignedCount++
		}

		// Bug vs Feature classification
		isBugIssue := false
		isFeatureIssue := false
		for _, label := range issue.Labels {
			labelName := strings.ToLower(label.GetName())
			if strings.Contains(labelName, "bug") {
				isBugIssue = true
			}
			if strings.Contains(labelName, "feature") || strings.Contains(labelName, "enhancement") {
				isFeatureIssue = true
			}
		}
		if isBugIssue {
			bugCount++
		}
		if isFeatureIssue {
			featureCount++
		}
	}

	// Lifetime calculation
	var totalLifetime time.Duration
	var issuesWithLinkedPR int

	for _, issue := range closedIssues {
		if issue.GetClosedAt().IsZero() {
			continue
		}
		lifetime := issue.GetClosedAt().Time.Sub(issue.GetCreatedAt().Time)
		totalLifetime += lifetime

		// Check if issue has linked PR
		if issue.PullRequestLinks != nil {
			issuesWithLinkedPR++
		}
	}

	// Calculate Time to First Response (sample to avoid excessive API calls)
	sampleLimit := 10
	if cfg.IncludeDeep {
		sampleLimit = 30
	}
	if len(closedIssues) < sampleLimit {
		sampleLimit = len(closedIssues)
	}

	for i := 0; i < sampleLimit; i++ {
		issue := closedIssues[i]
		comments, err := client.GetIssueComments(ctx, repo.Owner, repo.Name, issue.GetNumber(), nil)
		if err == nil && len(comments) > 0 {
			firstComment := comments[0]
			responseTime := firstComment.GetCreatedAt().Sub(issue.GetCreatedAt().Time)
			if responseTime > 0 {
				totalResponseTime += responseTime
				responseCount++
			}
		}
	}

	avgLifetimeHours := 0.0
	if len(closedIssues) > 0 {
		avgLifetimeHours = totalLifetime.Hours() / float64(len(closedIssues))
	}

	avgResponseHours := 0.0
	if responseCount > 0 {
		avgResponseHours = totalResponseTime.Hours() / float64(responseCount)
	}

	assigneeRatio := 0.0
	if len(openIssues) > 0 {
		assigneeRatio = float64(assignedCount) / float64(len(openIssues))
	}

	issueWithPRRatio := 0.0
	if len(closedIssues) > 0 {
		issueWithPRRatio = float64(issuesWithLinkedPR) / float64(len(closedIssues))
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
		{Key: "open_issues_total", Value: float64(len(openIssues)), DisplayValue: fmt.Sprintf("%d", len(openIssues)), Description: "Total open issues"},
		{Key: "closed_issues_in_window", Value: float64(len(closedIssues)), DisplayValue: fmt.Sprintf("%d", len(closedIssues)), Description: "Issues closed in window"},
		{Key: "stale_issues", Value: float64(staleCount), DisplayValue: fmt.Sprintf("%d", staleCount), Description: "Inactive issues beyond threshold"},
		{Key: "zombie_issues", Value: float64(zombieCount), DisplayValue: fmt.Sprintf("%d", zombieCount), Description: "Very old open issues"},
		{Key: "avg_issue_lifetime", Value: avgLifetimeHours, Unit: "hours", DisplayValue: fmt.Sprintf("%.1fh", avgLifetimeHours), Description: "Average time to close"},
		{Key: "avg_first_response_time", Value: avgResponseHours, Unit: "hours", DisplayValue: fmt.Sprintf("%.1fh", avgResponseHours), Description: "Average time to first comment"},
		{Key: "label_coverage", Value: labeledRatio, Unit: "percent", DisplayValue: fmt.Sprintf("%.0f%%", labeledRatio*100), Description: "% issues with labels"},
		{Key: "assignee_coverage", Value: assigneeRatio, Unit: "percent", DisplayValue: fmt.Sprintf("%.0f%%", assigneeRatio*100), Description: "% open issues assigned"},
		{Key: "issue_pr_link_rate", Value: issueWithPRRatio, Unit: "percent", DisplayValue: fmt.Sprintf("%.0f%%", issueWithPRRatio*100), Description: "% closed issues with linked PRs"},
		{Key: "bug_count", Value: float64(bugCount), DisplayValue: fmt.Sprintf("%d", bugCount), Description: "Open bugs"},
		{Key: "feature_count", Value: float64(featureCount), DisplayValue: fmt.Sprintf("%d", featureCount), Description: "Open feature requests"},
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
