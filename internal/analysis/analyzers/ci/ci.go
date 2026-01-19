package ci

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct {
}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Name() string {
	return "ci"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	result := models.AnalyzerResult{Name: "ci"}

	// List workflow runs created since cfg.Since
	// Note: ListRepositoryWorkflowRuns supports filtering by creation time via query string conceptually
	// but the library options use specific fields. We can check created_range if library supports it or filter locally.
	// The library opts has 'Created' string. "2023-01-01..*"

	sinceStr := fmt.Sprintf(">=%s", cfg.Since.Format("2006-01-02"))
	opts := &github.ListWorkflowRunsOptions{
		Created: sinceStr,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// We might need to page to get all runs in window
	// For simplicity in this iteration, let's just get the first page (up to 100 runs) or loop a few times.
	// Users with massive CI churn might exceed this, but it's a start.

	var allRuns []*github.WorkflowRun
	for {
		runs, resp, err := client.GetWorkflowRuns(ctx, repo.Owner, repo.Name, opts)
		if err != nil {
			return result, err
		}

		allRuns = append(allRuns, runs.WorkflowRuns...)

		if resp.NextPage == 0 || len(allRuns) >= 500 { // Limit to 500 runs to avoid hitting limits hard
			break
		}
		opts.Page = resp.NextPage
	}

	if len(allRuns) == 0 {
		return result, nil
	}

	// Calculate Metrics
	var (
		totalRuns            int
		successCount         int
		failureCount         int
		totalDuration        time.Duration
		workflowCounts       = make(map[string]int)
		workflowSuccess      = make(map[string]int)
		workflowFail         = make(map[string]int)
		workflowRuntime      = make(map[string]time.Duration) // Accumulate runtime per workflow
		workflowRuntimeCount = make(map[string]int)           // Count successful runs for averaging
	)

	for _, run := range allRuns {
		// Filter out runs that started before Since just in case API returned strictly older ones
		if run.CreatedAt.Before(cfg.Since) {
			continue
		}

		totalRuns++
		wfName := run.GetName()
		workflowCounts[wfName]++

		conclusion := run.GetConclusion()
		// statuses: success, failure, neutral, cancelled, timed_out, action_required

		switch conclusion {
		case "success":
			successCount++
			workflowSuccess[wfName]++

			// Calculate duration
			start := run.GetCreatedAt().Time
			end := run.GetUpdatedAt().Time // UpdatedAt is usually close to finished for completed runs
			if !start.IsZero() && !end.IsZero() {
				d := end.Sub(start)
				if d > 0 {
					totalDuration += d
					workflowRuntime[wfName] += d
					workflowRuntimeCount[wfName]++
				}
			}

		case "failure", "timed_out", "startup_failure":
			failureCount++
			workflowFail[wfName]++
		}
	}

	successRate := 0.0
	if totalRuns > 0 {
		successRate = float64(successCount) / float64(totalRuns)
	}

	avgDurationSeconds := 0.0
	if successCount > 0 {
		avgDurationSeconds = totalDuration.Seconds() / float64(successCount)
	}

	// Metrics
	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "workflow_runs_total",
		Value:        float64(totalRuns),
		DisplayValue: fmt.Sprintf("%d", totalRuns),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "success_rate",
		Value:        successRate * 100,
		Unit:         "percent",
		DisplayValue: fmt.Sprintf("%.1f%%", successRate*100),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "avg_runtime",
		Value:        avgDurationSeconds,
		Unit:         "seconds",
		DisplayValue: (time.Duration(avgDurationSeconds) * time.Second).String(),
	})

	// Findings

	// 1. High Failure Rate Detection
	if totalRuns > 10 && successRate < 0.80 {
		result.Findings = append(result.Findings, models.Finding{
			Type:     "ci_stability",
			Severity: models.SeverityHigh,
			Message:  fmt.Sprintf("Global success rate is low (%.0f%%). CI may be unstable.", successRate*100),
		})
	}

	// 2. Identify Flaky/Failing Workflows
	for name, count := range workflowCounts {
		if count < 5 {
			continue
		}
		fails := workflowFail[name]
		rate := float64(fails) / float64(count)

		if rate > 0.4 {
			result.Findings = append(result.Findings, models.Finding{
				Type:     "flaky_workflow",
				Severity: models.SeverityMedium,
				Message:  fmt.Sprintf("Workflow '%s' fails often (%.0f%% failure rate).", name, rate*100),
			})
		}
	}

	// 3. Slow Builds
	if avgDurationSeconds > 900 { // 15 mins
		result.Findings = append(result.Findings, models.Finding{
			Type:     "slow_builds",
			Severity: models.SeverityMedium,
			Message:  fmt.Sprintf("Average build time is high (%s). Consider optimization.", (time.Duration(avgDurationSeconds) * time.Second).String()),
		})
	}

	// 4. Most Expensive Workflow
	var maxWfName string
	var maxWfAvg float64

	for name, totalTime := range workflowRuntime {
		count := workflowRuntimeCount[name]
		if count > 0 {
			avg := totalTime.Seconds() / float64(count)
			if avg > maxWfAvg {
				maxWfAvg = avg
				maxWfName = name
			}
		}
	}

	if maxWfName != "" && maxWfAvg > 300 { // Only report if longer than 5 mins
		result.Findings = append(result.Findings, models.Finding{
			Type:     "expensive_workflow",
			Severity: models.SeverityInfo,
			Message:  fmt.Sprintf("Most expensive workflow: '%s' (avg %s).", maxWfName, (time.Duration(maxWfAvg) * time.Second).String()),
		})
	}

	return result, nil
}
