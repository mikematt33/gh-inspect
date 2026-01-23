package prflow

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
	return "pr-flow"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	// 1. Fetch all recent PRs in one call (both open and closed) to avoid multiple API calls
	// We'll filter by state in memory
	opts := &github.PullRequestListOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
		Sort:        "updated",
		Direction:   "desc",
	}

	allPRs, err := client.GetPullRequests(ctx, repo.Owner, repo.Name, opts)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}

	// Filter by Config.Since and separate by state
	var recentClosedPRs []*github.PullRequest
	var openPRs []*github.PullRequest
	for _, pr := range allPRs {
		if pr.UpdatedAt != nil && pr.UpdatedAt.After(cfg.Since) {
			if pr.GetState() == "closed" {
				recentClosedPRs = append(recentClosedPRs, pr)
			} else if pr.GetState() == "open" {
				openPRs = append(openPRs, pr)
			}
		}
	}

	var totalMergeTime time.Duration
	var mergedCount int
	var totalClosed = len(recentClosedPRs)
	var selfMergeCount int
	var draftPRCount int
	var hasDescriptionCount int

	for _, pr := range recentClosedPRs {
		if pr.MergedAt != nil {
			mergedCount++
			totalMergeTime += pr.MergedAt.Sub(pr.CreatedAt.Time)

			// Check self-merge (author == merger)
			if pr.User != nil && pr.MergedBy != nil {
				if pr.User.GetLogin() == pr.MergedBy.GetLogin() {
					selfMergeCount++
				}
			}
		}

		// Check if PR was a draft
		if pr.GetDraft() {
			draftPRCount++
		}

		// Check description quality (has meaningful body)
		if len(pr.GetBody()) > 50 {
			hasDescriptionCount++
		}
	}

	// Metrics Calculation
	var metrics []models.Metric
	var sizeFindings []models.Finding // Local findings for size analysis

	// 2. Use already fetched PRs for "Time to First Review" (avoid duplicate API call)
	// Sample from the PRs we already have instead of fetching again
	samplePRs := allPRs
	if len(samplePRs) > 0 {
		// Calculate Time To First Review
		// This requires N+1 queries per PR. Limit aggressively to minimize API calls.
		// Deep scan: 20 PRs, Normal scan: 5 PRs

		limitChecks := 5
		if cfg.IncludeDeep {
			limitChecks = 20
		}

		var totalReviewTime time.Duration
		var reviewCount int
		var totalApprovals int
		var prsWithReviews int

		for i, pr := range samplePRs {
			if i >= limitChecks {
				break
			}
			reviews, err := client.GetReviews(ctx, repo.Owner, repo.Name, pr.GetNumber(), nil)
			if err != nil {
				continue
			}
			if len(reviews) > 0 {
				prsWithReviews++
				firstReview := reviews[0].SubmittedAt
				if firstReview.After(pr.CreatedAt.Time) {
					totalReviewTime += firstReview.Sub(pr.CreatedAt.Time)
					reviewCount++
				}

				// Count approvals
				for _, review := range reviews {
					if review.GetState() == "APPROVED" {
						totalApprovals++
					}
				}
			}
		}

		if reviewCount > 0 {
			avgReview := totalReviewTime / time.Duration(reviewCount)
			avgReviewTimeHours := avgReview.Hours()

			metrics = append(metrics, models.Metric{
				Key:          "avg_time_to_first_review",
				Value:        avgReviewTimeHours,
				Unit:         "hours",
				DisplayValue: fmt.Sprintf("%.1fh", avgReviewTimeHours),
				Description:  "Average time until first review",
			})
		}

		if prsWithReviews > 0 {
			avgApprovals := float64(totalApprovals) / float64(prsWithReviews)

			metrics = append(metrics, models.Metric{
				Key:          "avg_approvals_per_pr",
				Value:        avgApprovals,
				Unit:         "count",
				DisplayValue: fmt.Sprintf("%.1f", avgApprovals),
				Description:  "Average number of approvals per PR",
			})
		}
	}

	if mergedCount > 0 {
		avgTime := totalMergeTime / time.Duration(mergedCount)
		metrics = append(metrics, models.Metric{
			Key:          "avg_cycle_time_hours",
			Value:        avgTime.Hours(),
			Unit:         "hours",
			DisplayValue: fmt.Sprintf("%.1fh", avgTime.Hours()),
		})

		// PR Size Distribution
		// Note: ListPullRequests response PR objects might not always have Additions/Deletions populated
		// unless fetched individually in some API versions.
		// However, if we are sampling, we can check.
		// If fields are zero, we might be hitting that limitation.
		// But let's check our recentClosedPRs.

		var totalAdditions, totalDeletions int
		var prsWithData int

		for _, pr := range recentClosedPRs {
			// Check if we have size data (Additions or Deletions > 0 or ChangedFiles > 0)
			// Usually list endpoints don't return these stats. We need get single PR for that.
			// Since we want "Size Distribution", and doing it for ALL 100 PRs is expensive...
			// Let's create a finding if we have "Huge PRs" based on a small sample or if available.
			// NOTE: google/go-github docs say Additions/Deletions are available on GetSingle, not List.

			// Optimization: We won't fetch individual PRs here to respect rate limits.
			// We'll rely on what we have. If 0, we skip.
			if pr.Additions != nil {
				totalAdditions += *pr.Additions
				totalDeletions += *pr.Deletions
				prsWithData++
			}
		}

		// If list doesn't return size (likely), we might scan just the top 5 recently merged to check for "Giant PRs"
		// This fits "Opinionated Insights".

		if prsWithData == 0 && len(recentClosedPRs) > 0 {
			// Sample top 5 merged PRs for size data
			// Only fetch individual PRs if absolutely necessary (list doesn't have size data)
			limit := 5
			if len(recentClosedPRs) < limit {
				limit = len(recentClosedPRs)
			}

			for i := 0; i < limit; i++ {
				prNum := recentClosedPRs[i].GetNumber()
				fullPR, err := client.GetPullRequest(ctx, repo.Owner, repo.Name, prNum)
				if err == nil {
					adds := fullPR.GetAdditions()
					dels := fullPR.GetDeletions()
					total := adds + dels

					if total > 1000 {
						sizeFindings = append(sizeFindings, models.Finding{
							Type:        "giant_pr",
							Severity:    models.SeverityInfo,
							Message:     fmt.Sprintf("Large PR detected: #%d has %d changes. Large PRs slow down review.", prNum, total),
							Actionable:  true,
							Remediation: "Split PR into smaller chunks.",
						})
					}

					totalAdditions += adds
					totalDeletions += dels
					prsWithData++
				}
			}
		}

		if prsWithData > 0 {
			avgSize := (totalAdditions + totalDeletions) / prsWithData
			metrics = append(metrics, models.Metric{
				Key:          "avg_pr_size_lines",
				Value:        float64(avgSize),
				Unit:         "lines",
				DisplayValue: fmt.Sprintf("%d LOC", avgSize),
				Description:  "Average lines changed (add+del) per PR (sampled)",
			})
		}
	}

	if totalClosed > 0 {
		ratio := float64(mergedCount) / float64(totalClosed)
		metrics = append(metrics, models.Metric{
			Key:          "merge_ratio",
			Value:        ratio * 100,
			Unit:         "percent",
			DisplayValue: fmt.Sprintf("%.0f%%", ratio*100),
			Description:  "Percentage of closed PRs that were merged",
		})

		if mergedCount > 0 {
			selfMergeRate := float64(selfMergeCount) / float64(mergedCount) * 100
			metrics = append(metrics, models.Metric{
				Key:          "self_merge_rate",
				Value:        selfMergeRate,
				Unit:         "percent",
				DisplayValue: fmt.Sprintf("%.0f%%", selfMergeRate),
				Description:  "Percentage of PRs merged by their author",
			})
		}

		draftRate := float64(draftPRCount) / float64(totalClosed) * 100
		metrics = append(metrics, models.Metric{
			Key:          "draft_pr_rate",
			Value:        draftRate,
			Unit:         "percent",
			DisplayValue: fmt.Sprintf("%.0f%%", draftRate),
			Description:  "Percentage of PRs started as draft",
		})

		descriptionRate := float64(hasDescriptionCount) / float64(totalClosed) * 100
		metrics = append(metrics, models.Metric{
			Key:          "pr_description_quality",
			Value:        descriptionRate,
			Unit:         "percent",
			DisplayValue: fmt.Sprintf("%.0f%%", descriptionRate),
			Description:  "Percentage of PRs with meaningful descriptions",
		})
	}

	// 3. Stale PRs (Findings) - use already fetched open PRs
	var findings []models.Finding
	now := time.Now()

	for _, pr := range openPRs {
		if pr.UpdatedAt == nil {
			continue
		}
		daysSinceUpdate := now.Sub(pr.UpdatedAt.Time).Hours() / 24

		if int(daysSinceUpdate) > a.StaleThresholdDays {
			findings = append(findings, models.Finding{
				Type:        "stale_pr",
				Severity:    models.SeverityMedium,
				Message:     fmt.Sprintf("PR has been inactive for > %d days", a.StaleThresholdDays),
				Location:    pr.GetHTMLURL(),
				Actionable:  true,
				Remediation: "Ping the reviewer or close the PR.",
			})
		}
	}

	// Merge findings
	findings = append(findings, sizeFindings...)

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
