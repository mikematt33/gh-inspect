package actions

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

// Fetcher is the subset of the GitHub Actions client the engine relies on.
// It is satisfied by *github.ActionsClient and is mockable in tests.
type Fetcher interface {
	ListWorkflows(ctx context.Context, owner, repo string) ([]*github.Workflow, error)
	ListWorkflowRuns(ctx context.Context, owner, repo, created string, maxRuns int) ([]*github.WorkflowRun, int, error)
	ListWorkflowJobs(ctx context.Context, owner, repo string, runID int64) ([]*github.WorkflowJob, error)
	GetFileContent(ctx context.Context, owner, repo, path string) (string, error)
}

// Options controls the scope and thresholds of an Actions scan.
type Options struct {
	Days                 int     // lookback window in days
	MaxRuns              int     // cap on runs fetched per repo (0 = engine default)
	WorkflowFilter       string  // restrict to a single workflow by file name or display name
	Top                  int     // top-N slowest jobs / failure hotspots to report
	DurationThresholdSec float64 // flag workflows whose avg duration exceeds this
	QueueThresholdSec    float64 // flag workflows whose avg queue time exceeds this
	SampleJobRuns        int     // recent runs per workflow to inspect for job/runner data (0 disables)
	FlakinessThreshold   float64 // flakiness score above which a workflow is flagged flaky
}

// withDefaults returns a copy of opts with sane defaults applied.
func (o Options) withDefaults() Options {
	if o.Days <= 0 {
		o.Days = 30
	}
	if o.MaxRuns <= 0 {
		o.MaxRuns = 1000
	}
	if o.Top <= 0 {
		o.Top = 5
	}
	if o.DurationThresholdSec <= 0 {
		o.DurationThresholdSec = 1800 // 30 minutes
	}
	if o.QueueThresholdSec <= 0 {
		o.QueueThresholdSec = 300 // 5 minutes
	}
	if o.FlakinessThreshold <= 0 {
		o.FlakinessThreshold = 0.3
	}
	// SampleJobRuns defaults to 0 (disabled) to keep API cost predictable.
	return o
}

// Engine scans repositories and computes GitHub Actions analytics.
type Engine struct {
	client Fetcher
}

// NewEngine constructs an Engine backed by the given Fetcher.
func NewEngine(client Fetcher) *Engine {
	return &Engine{client: client}
}

// ScanRepo computes Actions analytics for a single repository.
func (e *Engine) ScanRepo(ctx context.Context, owner, repo string, opts Options) (models.ActionsRepoReport, error) {
	opts = opts.withDefaults()
	report := models.ActionsRepoReport{
		Name:      fmt.Sprintf("%s/%s", owner, repo),
		URL:       fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		RunnerMix: models.RunnerUsage{ByOS: map[string]int{}, ByLabel: map[string]int{}},
	}

	workflows, err := e.client.ListWorkflows(ctx, owner, repo)
	if err != nil {
		return report, fmt.Errorf("listing workflows: %w", err)
	}

	// Resolve triggers and apply the optional workflow filter.
	wfByID := map[int64]*github.Workflow{}
	triggersByID := map[int64]workflowTriggers{}
	selected := map[int64]bool{}
	for _, wf := range workflows {
		id := wf.GetID()
		if opts.WorkflowFilter != "" && !workflowMatches(wf, opts.WorkflowFilter) {
			continue
		}
		wfByID[id] = wf
		selected[id] = true
		if raw, ferr := e.client.GetFileContent(ctx, owner, repo, wf.GetPath()); ferr == nil {
			triggersByID[id] = parseWorkflowTriggers(raw)
		}
	}

	since := time.Now().AddDate(0, 0, -opts.Days)
	created := fmt.Sprintf(">=%s", since.Format("2006-01-02"))
	runs, _, err := e.client.ListWorkflowRuns(ctx, owner, repo, created, opts.MaxRuns)
	if err != nil {
		return report, fmt.Errorf("listing workflow runs: %w", err)
	}

	// Group runs by workflow, keeping only selected workflows within the window.
	runsByWF := map[int64][]*github.WorkflowRun{}
	for _, run := range runs {
		if !selected[run.GetWorkflowID()] {
			continue
		}
		if run.GetCreatedAt().Before(since) {
			continue
		}
		runsByWF[run.GetWorkflowID()] = append(runsByWF[run.GetWorkflowID()], run)
	}

	for id, wf := range wfByID {
		wfRuns := runsByWF[id]
		wr := e.buildWorkflowReport(ctx, owner, repo, wf, triggersByID[id], wfRuns, opts, &report.RunnerMix)
		report.Workflows = append(report.Workflows, wr)

		// Scheduled monitoring.
		if containsEvent(triggersByID[id].Events, "schedule") {
			report.Scheduled = append(report.Scheduled, buildScheduledSummary(wf.GetName(), triggersByID[id].Crons, wfRuns))
		}
	}

	sort.SliceStable(report.Workflows, func(i, j int) bool {
		return report.Workflows[i].TotalRuns > report.Workflows[j].TotalRuns
	})

	report.Totals = computeRepoTotals(report.Workflows, report.RunnerMix)
	report.Totals.WorkflowCount = len(report.Workflows)
	return report, nil
}

// buildWorkflowReport computes all per-workflow metrics from its runs.
func (e *Engine) buildWorkflowReport(ctx context.Context, owner, repo string, wf *github.Workflow, trig workflowTriggers, runs []*github.WorkflowRun, opts Options, runnerMix *models.RunnerUsage) models.WorkflowReport {
	wr := models.WorkflowReport{
		Name:     wf.GetName(),
		Path:     wf.GetPath(),
		State:    wf.GetState(),
		Triggers: trig.Events,
		URL:      wf.GetHTMLURL(),
	}

	// Order chronologically (oldest first) for trend/flakiness/MTBF.
	chrono := make([]*github.WorkflowRun, len(runs))
	copy(chrono, runs)
	sort.SliceStable(chrono, func(i, j int) bool {
		return chrono[i].GetCreatedAt().Before(chrono[j].GetCreatedAt().Time)
	})

	var durations, queueTimes []float64
	var conclusionsChrono []string
	var failureTimes []time.Time
	var maxQueue float64

	for _, run := range chrono {
		switch run.GetConclusion() {
		case "success":
			wr.Success++
		case "failure", "timed_out", "startup_failure":
			wr.Failure++
			failureTimes = append(failureTimes, run.GetCreatedAt().Time)
		case "cancelled":
			wr.Cancelled++
		case "skipped":
			wr.Skipped++
		}
		conclusionsChrono = append(conclusionsChrono, run.GetConclusion())

		if d, ok := runDurationSec(run); ok {
			durations = append(durations, d)
		}
		if q, ok := queueSec(run); ok {
			queueTimes = append(queueTimes, q)
			if q > maxQueue {
				maxQueue = q
			}
		}
	}

	wr.TotalRuns = len(chrono)
	if wr.TotalRuns > 0 {
		wr.SuccessRate = float64(wr.Success) / float64(wr.TotalRuns) * 100
	}

	wr.AvgDurationSec = mean(durations)
	wr.MedianDurationSec = median(durations)
	wr.P95DurationSec = percentile(durations, 95)
	wr.MaxDurationSec = maxOf(durations)
	wr.DurationTrendSec = durationTrendSec(durations)
	wr.MTBFHours = mtbfHours(failureTimes)
	wr.FlakinessScore = flakiness(conclusionsChrono)
	wr.Flaky = wr.FlakinessScore >= opts.FlakinessThreshold && wr.TotalRuns >= 4
	wr.AvgQueueSec = mean(queueTimes)
	wr.MaxQueueSec = maxQueue

	// Job-level breakdown and runner usage from a sample of recent runs.
	if opts.SampleJobRuns > 0 && wr.TotalRuns > 0 {
		e.analyzeJobs(ctx, owner, repo, chrono, opts, &wr, runnerMix)
	}

	wr.Findings = workflowFindings(wr, opts)
	return wr
}

// analyzeJobs fetches jobs for the most recent sample of runs to compute
// per-job durations, slowest jobs, and runner usage.
func (e *Engine) analyzeJobs(ctx context.Context, owner, repo string, chrono []*github.WorkflowRun, opts Options, wr *models.WorkflowReport, runnerMix *models.RunnerUsage) {
	// Most recent runs first.
	recent := make([]*github.WorkflowRun, len(chrono))
	copy(recent, chrono)
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].GetCreatedAt().After(recent[j].GetCreatedAt().Time)
	})
	if len(recent) > opts.SampleJobRuns {
		recent = recent[:opts.SampleJobRuns]
	}

	jobDur := map[string][]float64{}
	for _, run := range recent {
		jobs, err := e.client.ListWorkflowJobs(ctx, owner, repo, run.GetID())
		if err != nil {
			continue
		}
		for _, job := range jobs {
			rc := classifyRunner(job.Labels)
			if rc.hosted {
				runnerMix.GitHubHosted++
				wr.Runners.GitHubHosted++
			} else {
				runnerMix.SelfHosted++
				wr.Runners.SelfHosted++
			}
			runnerMix.ByOS[rc.os]++
			if rc.primary != "" {
				runnerMix.ByLabel[rc.primary]++
			}
			if rc.deprecated && rc.primary != "" {
				runnerMix.Deprecated = appendUnique(runnerMix.Deprecated, rc.primary)
			}
			if d, ok := jobDurationSec(job); ok {
				jobDur[job.GetName()] = append(jobDur[job.GetName()], d)
			}
		}
	}

	for name, ds := range jobDur {
		wr.SlowestJobs = append(wr.SlowestJobs, models.JobDurationSummary{
			Name:           name,
			AvgDurationSec: mean(ds),
			MaxDurationSec: maxOf(ds),
			Samples:        len(ds),
		})
	}
	sort.SliceStable(wr.SlowestJobs, func(i, j int) bool {
		return wr.SlowestJobs[i].AvgDurationSec > wr.SlowestJobs[j].AvgDurationSec
	})
	if len(wr.SlowestJobs) > opts.Top {
		wr.SlowestJobs = wr.SlowestJobs[:opts.Top]
	}
}

// workflowMatches reports whether a workflow matches the user-provided filter,
// which may be a file name (e.g. "ci.yml"), a path, or a display name.
func workflowMatches(wf *github.Workflow, filter string) bool {
	f := strings.ToLower(filter)
	if strings.EqualFold(wf.GetName(), filter) {
		return true
	}
	p := strings.ToLower(wf.GetPath())
	if p == f || strings.HasSuffix(p, "/"+f) || path.Base(p) == f {
		return true
	}
	return false
}

func containsEvent(events []string, target string) bool {
	for _, e := range events {
		if e == target {
			return true
		}
	}
	return false
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

// runDurationSec returns a run's wall-clock duration in seconds, preferring the
// run start time over the created (queued) time.
func runDurationSec(run *github.WorkflowRun) (float64, bool) {
	end := run.GetUpdatedAt().Time
	start := run.GetRunStartedAt().Time
	if start.IsZero() {
		start = run.GetCreatedAt().Time
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0, false
	}
	return end.Sub(start).Seconds(), true
}

// queueSec returns the time a run waited between being queued and starting.
func queueSec(run *github.WorkflowRun) (float64, bool) {
	queued := run.GetCreatedAt().Time
	started := run.GetRunStartedAt().Time
	if queued.IsZero() || started.IsZero() || !started.After(queued) {
		return 0, false
	}
	return started.Sub(queued).Seconds(), true
}

// jobDurationSec returns a job's duration in seconds.
func jobDurationSec(job *github.WorkflowJob) (float64, bool) {
	start := job.GetStartedAt().Time
	end := job.GetCompletedAt().Time
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0, false
	}
	return end.Sub(start).Seconds(), true
}
