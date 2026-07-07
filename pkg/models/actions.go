package models

import "time"

// ActionsReport is the top-level output for the `actions` analytics command.
// It captures GitHub Actions analytics across one or more repositories.
type ActionsReport struct {
	Meta         ActionsMeta          `json:"meta"`
	Repositories []ActionsRepoReport  `json:"repositories"`
	Summary      ActionsOrgSummary    `json:"summary"`
	Preflight    *PreflightEstimate   `json:"preflight,omitempty"`
	TokenStatus  []TokenStatusSummary `json:"token_status,omitempty"`
}

// ActionsMeta contains metadata about the actions scan execution.
type ActionsMeta struct {
	GeneratedAt time.Time `json:"generated_at"`
	CLIVersion  string    `json:"cli_version"`
	Scope       string    `json:"scope"` // "repo", "multi-repo", or "org"
	WindowDays  int       `json:"window_days"`
	Duration    string    `json:"duration"`
}

// ActionsRepoReport holds Actions analytics for a single repository.
type ActionsRepoReport struct {
	Name      string             `json:"name"` // owner/repo
	URL       string             `json:"url"`
	Workflows []WorkflowReport   `json:"workflows"`
	RunnerMix RunnerUsage        `json:"runner_mix"`
	Totals    RepoActionsTotals  `json:"totals"`
	Scheduled []ScheduledSummary `json:"scheduled,omitempty"`
}

// RepoActionsTotals aggregates run counts and timing for a repository.
type RepoActionsTotals struct {
	WorkflowCount  int     `json:"workflow_count"`
	TotalRuns      int     `json:"total_runs"`
	Success        int     `json:"success"`
	Failure        int     `json:"failure"`
	Cancelled      int     `json:"cancelled"`
	Skipped        int     `json:"skipped"`
	SuccessRate    float64 `json:"success_rate"` // 0-100
	AvgDurationSec float64 `json:"avg_duration_sec"`
	ComputeMinutes float64 `json:"compute_minutes"` // sum of run durations in minutes
}

// WorkflowReport holds per-workflow analytics.
type WorkflowReport struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	State    string   `json:"state"`    // active / disabled_manually / disabled_inactivity
	Triggers []string `json:"triggers"` // push, pull_request, schedule, workflow_dispatch, ...
	URL      string   `json:"url"`

	TotalRuns int `json:"total_runs"`
	Success   int `json:"success"`
	Failure   int `json:"failure"`
	Cancelled int `json:"cancelled"`
	Skipped   int `json:"skipped"`

	SuccessRate float64 `json:"success_rate"` // 0-100

	// Duration metrics (seconds)
	AvgDurationSec    float64 `json:"avg_duration_sec"`
	MedianDurationSec float64 `json:"median_duration_sec"`
	P95DurationSec    float64 `json:"p95_duration_sec"`
	MaxDurationSec    float64 `json:"max_duration_sec"`

	// Reliability
	MTBFHours      float64 `json:"mtbf_hours"`      // mean time between failures
	FlakinessScore float64 `json:"flakiness_score"` // 0-1, fraction of pass/fail transitions
	Flaky          bool    `json:"flaky"`

	// Trend: positive means getting slower (later half avg - earlier half avg, seconds)
	DurationTrendSec float64 `json:"duration_trend_sec"`

	// Queue time (seconds), if data available
	AvgQueueSec float64 `json:"avg_queue_sec"`
	MaxQueueSec float64 `json:"max_queue_sec"`

	SlowestJobs []JobDurationSummary `json:"slowest_jobs,omitempty"`

	// Runners summarizes the hosting type (GitHub-hosted vs self-hosted) of the
	// sampled jobs for this workflow. Empty when no job-level data was sampled.
	Runners RunnerUsage `json:"runners,omitempty"`

	Findings []Finding `json:"findings,omitempty"`
}

// JobDurationSummary summarizes timing for a job (or step) within a workflow.
type JobDurationSummary struct {
	Name           string  `json:"name"`
	AvgDurationSec float64 `json:"avg_duration_sec"`
	MaxDurationSec float64 `json:"max_duration_sec"`
	Samples        int     `json:"samples"`
}

// RunnerUsage breaks down runner usage by hosting type and OS.
type RunnerUsage struct {
	GitHubHosted int            `json:"github_hosted"` // job count on GitHub-hosted runners
	SelfHosted   int            `json:"self_hosted"`   // job count on self-hosted runners
	ByOS         map[string]int `json:"by_os"`         // linux/windows/macos/unknown -> job count
	ByLabel      map[string]int `json:"by_label"`      // raw runner label -> job count
	Deprecated   []string       `json:"deprecated,omitempty"`
}

// ScheduledSummary describes a schedule-triggered workflow's recent behavior.
type ScheduledSummary struct {
	Workflow       string     `json:"workflow"`
	LastRun        *time.Time `json:"last_run,omitempty"`
	LastConclusion string     `json:"last_conclusion,omitempty"`
	NextExpected   *time.Time `json:"next_expected,omitempty"`
	CronExprs      []string   `json:"cron_exprs,omitempty"`
}

// ActionsOrgSummary rolls up analytics across all scanned repositories.
type ActionsOrgSummary struct {
	ReposScanned         int              `json:"repos_scanned"`
	TotalWorkflows       int              `json:"total_workflows"`
	TotalRuns            int              `json:"total_runs"`
	OverallSuccessRate   float64          `json:"overall_success_rate"` // 0-100
	ComputeMinutesHosted float64          `json:"compute_minutes_hosted"`
	ComputeMinutesSelf   float64          `json:"compute_minutes_self"`
	FailureHotspots      []FailureHotspot `json:"failure_hotspots,omitempty"`
}

// FailureHotspot identifies a workflow with a high number of failures.
type FailureHotspot struct {
	Repo     string  `json:"repo"`
	Workflow string  `json:"workflow"`
	Failures int     `json:"failures"`
	Runs     int     `json:"runs"`
	FailRate float64 `json:"fail_rate"` // 0-100
}

// PreflightEstimate summarizes the projected cost of a scan and quota availability.
type PreflightEstimate struct {
	Repos              int                  `json:"repos"`
	EstimatedWorkflows int                  `json:"estimated_workflows"`
	EstimatedAPICalls  int                  `json:"estimated_api_calls"`
	AvailableQuota     int                  `json:"available_quota"`
	Sources            []TokenStatusSummary `json:"sources"`
	Sufficient         bool                 `json:"sufficient"`
}

// TokenStatusSummary reports the state of a single credential (PAT or App install).
type TokenStatusSummary struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // "pat" or "app"
	Remaining int    `json:"remaining"`
	Limit     int    `json:"limit"`
	Exhausted bool   `json:"exhausted"`
	Invalid   bool   `json:"invalid"`
}
