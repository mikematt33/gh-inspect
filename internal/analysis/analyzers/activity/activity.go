package activity

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Name() string {
	return "activity"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	// TIER 1: Commit Velocity & Bus Factor (Time-bounded)
	// This respects the cfg.Since window to avoid excessive API calls

	result := models.AnalyzerResult{Name: a.Name()}

	// Get repository metadata for stars/forks
	repoData, err := client.GetRepository(ctx, repo.Owner, repo.Name)
	if err != nil {
		return result, err
	}

	commits, err := client.ListCommitsSince(ctx, repo.Owner, repo.Name, cfg.Since)
	if err != nil {
		// Check if this is an empty repository error
		// GitHub returns 409 Conflict for empty repositories
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 409 {
			// Return empty metrics for empty repos instead of failing
			result.Metrics = append(result.Metrics, models.Metric{
				Key:          "commits_total",
				Value:        0,
				DisplayValue: "0",
			})
			result.Findings = append(result.Findings, models.Finding{
				Type:     "empty_repository",
				Severity: models.SeverityInfo,
				Message:  "Repository is empty (no commits)",
			})
			return result, nil
		}
		return result, err
	}

	totalCommits := float64(len(commits))
	days := time.Since(cfg.Since).Hours() / 24
	dailyVelocity := 0.0
	if days > 0 {
		dailyVelocity = totalCommits / days
	}

	// Bus Factor Calculation & New Contributor Detection
	authorCounts := make(map[string]int)
	firstSeen := make(map[string]time.Time)

	for _, c := range commits {
		var author string
		commitTime := cfg.Since

		if c.Commit != nil && c.Commit.Author != nil && c.Commit.Author.Date != nil {
			commitTime = c.Commit.Author.Date.Time
		}

		if c.Author != nil && c.Author.Login != nil {
			author = *c.Author.Login
		} else if c.Commit != nil && c.Commit.Author != nil && c.Commit.Author.Name != nil {
			author = *c.Commit.Author.Name
		}

		if author != "" {
			authorCounts[author]++
			if _, exists := firstSeen[author]; !exists {
				firstSeen[author] = commitTime
			}
		}
	}

	// Count new contributors
	newContributors := 0
	for _, firstCommit := range firstSeen {
		if firstCommit.After(cfg.Since) {
			newContributors++
		}
	}

	busFactor, topAuthors := calculateBusFactor(authorCounts, int(totalCommits))

	// Star and Fork metrics
	stars := repoData.GetStargazersCount()
	forks := repoData.GetForksCount()
	watchers := repoData.GetWatchersCount()

	metrics := []models.Metric{
		{
			Key:          "commits_total",
			Value:        totalCommits,
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%.0f", totalCommits),
			Description:  "Total commits in the lookback window",
		},
		{
			Key:          "commit_velocity_daily",
			Value:        dailyVelocity,
			Unit:         "commits/day",
			DisplayValue: fmt.Sprintf("%.1f/day", dailyVelocity),
			Description:  "Average commits per day",
		},
		{
			Key:          "bus_factor",
			Value:        float64(busFactor),
			Unit:         "authors",
			DisplayValue: fmt.Sprintf("%d", busFactor),
			Description:  "Number of authors accounting for 50% of commits",
		},
		{
			Key:          "active_contributors",
			Value:        float64(len(authorCounts)),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", len(authorCounts)),
			Description:  "Total distinct authors",
		},
		{
			Key:          "new_contributors",
			Value:        float64(newContributors),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", newContributors),
			Description:  "Contributors with first commit in window",
		},
		{
			Key:          "stars",
			Value:        float64(stars),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", stars),
			Description:  "Total repository stars",
		},
		{
			Key:          "forks",
			Value:        float64(forks),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", forks),
			Description:  "Total repository forks",
		},
		{
			Key:          "watchers",
			Value:        float64(watchers),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", watchers),
			Description:  "Repository watchers",
		},
	}

	// Findings
	var findings []models.Finding
	if busFactor == 1 && totalCommits > 10 {
		findings = append(findings, models.Finding{
			Type:        "bus_factor_risk",
			Severity:    models.SeverityHigh,
			Message:     "Single contributor risk: 50% of commits are by 1 person",
			Actionable:  true,
			Remediation: "Encourage code rotation and pair programming.",
		})
	}

	// Provide context in description about top authors
	if len(topAuthors) > 0 {
		// In the future, we can add a specific "finding" or metadata about who the top authors are.
		// For now, we leave this calculated but unused to avoid an empty branch lint error.
		_ = topAuthors
	}

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}

func calculateBusFactor(counts map[string]int, total int) (int, []string) {
	if total == 0 {
		return 0, nil
	}

	type authorCount struct {
		Name  string
		Count int
	}
	var sorted []authorCount
	for k, v := range counts {
		sorted = append(sorted, authorCount{k, v})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	accumulated := 0
	busFactor := 0
	var topAuthors []string

	for _, ac := range sorted {
		accumulated += ac.Count
		busFactor++
		topAuthors = append(topAuthors, ac.Name)
		if float64(accumulated)/float64(total) >= 0.5 {
			break
		}
	}
	return busFactor, topAuthors
}
