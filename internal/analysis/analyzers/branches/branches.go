package branches

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct {
	StaleThresholdDays int
}

func New(staleThresholdDays int) *Analyzer {
	return &Analyzer{
		StaleThresholdDays: staleThresholdDays,
	}
}

func (a *Analyzer) Name() string {
	return "branches"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	var metrics []models.Metric
	var findings []models.Finding

	// Get repository info for default branch
	repoInfo, err := client.GetRepository(ctx, repo.Owner, repo.Name)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}
	defaultBranch := repoInfo.GetDefaultBranch()

	// List all branches
	opts := &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100}}
	branches, _, err := client.GetUnderlyingClient().Repositories.ListBranches(ctx, repo.Owner, repo.Name, opts)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}

	totalBranches := len(branches)
	staleBranches := 0
	now := time.Now()

	// Check each branch for staleness (limit to first 50 to avoid rate limits)
	// Note: This only samples up to the first 50 branches. Repositories with more branches
	// may have additional stale branches not detected by this analyzer.
	limit := 50
	if len(branches) < limit {
		limit = len(branches)
	}

	for i := 0; i < limit; i++ {
		branch := branches[i]
		if branch.GetName() == defaultBranch {
			continue
		}

		// Get branch details to check last commit date
		branchDetail, _, err := client.GetUnderlyingClient().Repositories.GetBranch(ctx, repo.Owner, repo.Name, branch.GetName(), 0)
		if err != nil {
			continue
		}

		if branchDetail.Commit != nil && branchDetail.Commit.Commit != nil && branchDetail.Commit.Commit.Author != nil {
			lastCommitDate := branchDetail.Commit.Commit.Author.GetDate()
			daysSinceUpdate := now.Sub(lastCommitDate.Time).Hours() / 24

			if int(daysSinceUpdate) > a.StaleThresholdDays {
				staleBranches++
			}
		}
	}

	metrics = append(metrics, models.Metric{
		Key:          "total_branches",
		Value:        float64(totalBranches),
		DisplayValue: fmt.Sprintf("%d", totalBranches),
		Description:  "Total number of branches",
	})
	metrics = append(metrics, models.Metric{
		Key:          "stale_branches",
		Value:        float64(staleBranches),
		DisplayValue: fmt.Sprintf("%d", staleBranches),
		Description:  fmt.Sprintf("Branches inactive > %d days (sampled from first %d branches)", a.StaleThresholdDays, limit),
	})

	// Findings
	if totalBranches > 50 {
		findings = append(findings, models.Finding{
			Type:        "too_many_branches",
			Severity:    models.SeverityLow,
			Message:     fmt.Sprintf("Repository has %d branches", totalBranches),
			Actionable:  true,
			Remediation: "Clean up merged or stale branches.",
		})
	}

	if staleBranches > 10 {
		findings = append(findings, models.Finding{
			Type:        "stale_branches",
			Severity:    models.SeverityMedium,
			Message:     fmt.Sprintf("%d branches haven't been updated in %d+ days", staleBranches, a.StaleThresholdDays),
			Actionable:  true,
			Remediation: "Delete or merge stale branches.",
		})
	}

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
