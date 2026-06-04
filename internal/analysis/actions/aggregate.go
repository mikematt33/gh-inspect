package actions

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

// computeRepoTotals aggregates per-workflow stats into a repository roll-up.
func computeRepoTotals(workflows []models.WorkflowReport, mix models.RunnerUsage) models.RepoActionsTotals {
	var t models.RepoActionsTotals
	var durationWeightedSum float64
	for _, wf := range workflows {
		t.TotalRuns += wf.TotalRuns
		t.Success += wf.Success
		t.Failure += wf.Failure
		t.Cancelled += wf.Cancelled
		t.Skipped += wf.Skipped
		// avg duration * runs ≈ total compute seconds for the workflow
		durationWeightedSum += wf.AvgDurationSec * float64(wf.TotalRuns)
	}
	if t.TotalRuns > 0 {
		t.SuccessRate = float64(t.Success) / float64(t.TotalRuns) * 100
		t.AvgDurationSec = durationWeightedSum / float64(t.TotalRuns)
	}
	t.ComputeMinutes = durationWeightedSum / 60
	return t
}

// buildScheduledSummary derives the last run, conclusion and next expected run
// for a schedule-triggered workflow.
func buildScheduledSummary(workflow string, crons []string, runs []*github.WorkflowRun) models.ScheduledSummary {
	s := models.ScheduledSummary{Workflow: workflow, CronExprs: crons}

	var last *github.WorkflowRun
	for _, run := range runs {
		if run.GetEvent() != "schedule" {
			continue
		}
		if last == nil || run.GetCreatedAt().After(last.GetCreatedAt().Time) {
			last = run
		}
	}
	if last != nil {
		t := last.GetCreatedAt().Time
		s.LastRun = &t
		s.LastConclusion = last.GetConclusion()
	}

	// Next expected run: soonest activation across all cron expressions.
	now := time.Now().UTC()
	var next time.Time
	for _, expr := range crons {
		sched, err := parseCron(expr)
		if err != nil {
			continue
		}
		n := sched.Next(now)
		if n.IsZero() {
			continue
		}
		if next.IsZero() || n.Before(next) {
			next = n
		}
	}
	if !next.IsZero() {
		s.NextExpected = &next
	}
	return s
}

// BuildOrgSummary rolls up repository reports into an organization-level summary
// including compute minutes split by runner hosting and failure hotspots.
func BuildOrgSummary(repos []models.ActionsRepoReport, top int) models.ActionsOrgSummary {
	if top <= 0 {
		top = 5
	}
	var s models.ActionsOrgSummary
	s.ReposScanned = len(repos)

	var totalSuccess int
	var hotspots []models.FailureHotspot

	for _, repo := range repos {
		s.TotalWorkflows += repo.Totals.WorkflowCount
		s.TotalRuns += repo.Totals.TotalRuns
		totalSuccess += repo.Totals.Success

		// Split compute minutes by hosting using the repo's job-level runner mix.
		hostedJobs := repo.RunnerMix.GitHubHosted
		selfJobs := repo.RunnerMix.SelfHosted
		totalJobs := hostedJobs + selfJobs
		switch totalJobs {
		case 0:
			// No job-level data sampled: attribute all to hosted as a default.
			s.ComputeMinutesHosted += repo.Totals.ComputeMinutes
		default:
			hostedFrac := float64(hostedJobs) / float64(totalJobs)
			s.ComputeMinutesHosted += repo.Totals.ComputeMinutes * hostedFrac
			s.ComputeMinutesSelf += repo.Totals.ComputeMinutes * (1 - hostedFrac)
		}

		for _, wf := range repo.Workflows {
			if wf.Failure == 0 || wf.TotalRuns == 0 {
				continue
			}
			hotspots = append(hotspots, models.FailureHotspot{
				Repo:     repo.Name,
				Workflow: wf.Name,
				Failures: wf.Failure,
				Runs:     wf.TotalRuns,
				FailRate: float64(wf.Failure) / float64(wf.TotalRuns) * 100,
			})
		}
	}

	if s.TotalRuns > 0 {
		s.OverallSuccessRate = float64(totalSuccess) / float64(s.TotalRuns) * 100
	}

	sort.SliceStable(hotspots, func(i, j int) bool {
		return hotspots[i].Failures > hotspots[j].Failures
	})
	if len(hotspots) > top {
		hotspots = hotspots[:top]
	}
	s.FailureHotspots = hotspots
	return s
}

// workflowFindings generates qualitative findings for a workflow based on its
// computed metrics and the configured thresholds.
func workflowFindings(wr models.WorkflowReport, opts Options) []models.Finding {
	var findings []models.Finding

	if wr.State == "disabled_manually" || wr.State == "disabled_inactivity" {
		findings = append(findings, models.Finding{
			Type:     "workflow_disabled",
			Severity: models.SeverityInfo,
			Message:  fmt.Sprintf("Workflow %q is %s", wr.Name, wr.State),
		})
	}

	if wr.TotalRuns >= 10 && wr.SuccessRate < 80 {
		findings = append(findings, models.Finding{
			Type:        "low_success_rate",
			Severity:    models.SeverityHigh,
			Message:     fmt.Sprintf("%q has a low success rate (%.0f%% over %d runs)", wr.Name, wr.SuccessRate, wr.TotalRuns),
			Explanation: "A low success rate indicates unstable or flaky CI that wastes developer time and erodes trust in the pipeline.",
			SuggestedActions: []string{
				"Investigate the most frequent failure cause",
				"Quarantine or fix flaky tests",
			},
		})
	}

	if wr.Flaky {
		findings = append(findings, models.Finding{
			Type:        "flaky_workflow",
			Severity:    models.SeverityMedium,
			Message:     fmt.Sprintf("%q appears flaky (flakiness score %.2f)", wr.Name, wr.FlakinessScore),
			Explanation: "Workflows that frequently alternate between pass and fail usually have non-deterministic tests or environment issues.",
		})
	}

	if opts.DurationThresholdSec > 0 && wr.AvgDurationSec > opts.DurationThresholdSec {
		findings = append(findings, models.Finding{
			Type:        "slow_workflow",
			Severity:    models.SeverityMedium,
			Message:     fmt.Sprintf("%q averages %.0fs, exceeding the %.0fs threshold", wr.Name, wr.AvgDurationSec, opts.DurationThresholdSec),
			Explanation: "Long-running workflows slow feedback loops and increase compute cost.",
		})
	}

	if wr.DurationTrendSec > 0 && wr.AvgDurationSec > 0 && wr.DurationTrendSec/wr.AvgDurationSec > 0.25 {
		findings = append(findings, models.Finding{
			Type:        "duration_regression",
			Severity:    models.SeverityLow,
			Message:     fmt.Sprintf("%q is trending slower (+%.0fs recent vs. earlier)", wr.Name, wr.DurationTrendSec),
			Explanation: "A rising duration trend suggests growing test suites, added steps, or resource contention.",
		})
	}

	if opts.QueueThresholdSec > 0 && wr.AvgQueueSec > opts.QueueThresholdSec {
		findings = append(findings, models.Finding{
			Type:        "long_queue_time",
			Severity:    models.SeverityMedium,
			Message:     fmt.Sprintf("%q waits %.0fs in queue on average", wr.Name, wr.AvgQueueSec),
			Explanation: "Long queue times indicate runner contention or insufficient self-hosted runner capacity.",
		})
	}

	return findings
}
