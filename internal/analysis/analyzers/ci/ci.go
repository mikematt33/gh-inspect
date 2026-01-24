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

	// First, get the all-time total count (just 1 API call, 1 result to get total)
	allTimeOpts := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{
			PerPage: 1, // We only need the TotalCount, not the actual runs
		},
	}
	allTimeRuns, _, err := client.GetWorkflowRuns(ctx, repo.Owner, repo.Name, allTimeOpts)
	var allTimeTotal int
	if err == nil && allTimeRuns.TotalCount != nil {
		allTimeTotal = *allTimeRuns.TotalCount
	}

	// Now fetch runs within the time window for analysis
	sinceStr := fmt.Sprintf(">=%s", cfg.Since.Format("2006-01-02"))

	perPage := 100
	if cfg.DepthConfig.MaxWorkflowRuns > 0 && cfg.DepthConfig.MaxWorkflowRuns < 100 {
		perPage = cfg.DepthConfig.MaxWorkflowRuns
	}

	opts := &github.ListWorkflowRunsOptions{
		Created: sinceStr,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	// We might need to page to get all runs in window
	// Users can have many CI runs, so we'll fetch up to a reasonable limit
	// Use MaxWorkflowRuns from depth config
	var allRuns []*github.WorkflowRun
	var totalCount int // Actual total from API
	maxRuns := cfg.DepthConfig.MaxWorkflowRuns
	if maxRuns == 0 {
		// Fallback to old behavior if not configured
		maxRuns = 5000
	}

	for {
		runs, resp, err := client.GetWorkflowRuns(ctx, repo.Owner, repo.Name, opts)
		if err != nil {
			return result, err
		}

		// Capture total count from first response
		if totalCount == 0 && runs.TotalCount != nil {
			totalCount = *runs.TotalCount
		}

		allRuns = append(allRuns, runs.WorkflowRuns...)

		if resp.NextPage == 0 || len(allRuns) >= maxRuns {
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
		cancelledCount       int
		skippedCount         int
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
		// statuses: success, failure, neutral, cancelled, timed_out, action_required, skipped

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
		case "cancelled":
			cancelledCount++
		case "skipped", "neutral":
			skippedCount++
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
		Key:          "workflow_runs_all_time",
		Value:        float64(allTimeTotal),
		DisplayValue: fmt.Sprintf("%d", allTimeTotal),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "workflow_runs_in_window",
		Value:        float64(totalCount),
		DisplayValue: fmt.Sprintf("%d", totalCount),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "workflow_runs_analyzed",
		Value:        float64(len(allRuns)),
		DisplayValue: fmt.Sprintf("%d", len(allRuns)),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "unique_workflows",
		Value:        float64(len(workflowCounts)),
		DisplayValue: fmt.Sprintf("%d", len(workflowCounts)),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "success_count",
		Value:        float64(successCount),
		DisplayValue: fmt.Sprintf("%d", successCount),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "failure_count",
		Value:        float64(failureCount),
		DisplayValue: fmt.Sprintf("%d", failureCount),
	})

	result.Metrics = append(result.Metrics, models.Metric{
		Key:          "cancelled_count",
		Value:        float64(cancelledCount),
		DisplayValue: fmt.Sprintf("%d", cancelledCount),
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
			Type:        "ci_stability",
			Severity:    models.SeverityHigh,
			Message:     fmt.Sprintf("Global success rate is low (%.0f%%). CI may be unstable.", successRate*100),
			Explanation: "Low CI success rates indicate flaky tests, environmental issues, or unreliable builds that waste developer time.",
			SuggestedActions: []string{
				"Identify and fix the most frequently failing tests",
				"Add retry logic for flaky network-dependent tests",
			},
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
			Type:        "slow_builds",
			Severity:    models.SeverityMedium,
			Message:     fmt.Sprintf("Average build time is high (%s). Consider optimization.", (time.Duration(avgDurationSeconds) * time.Second).String()),
			Explanation: "Slow CI builds reduce developer productivity and increase feedback time, leading to context switching.",
			SuggestedActions: []string{
				"Cache dependencies between runs to speed up builds",
				"Parallelize test suites or split into multiple jobs",
			},
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
