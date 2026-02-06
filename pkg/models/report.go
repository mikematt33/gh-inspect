package models

import (
	"time"
)

// OutputMode defines how findings and insights are presented
type OutputMode string

const (
	// OutputModeSuggestive includes actionable recommendations and improvement tips
	OutputModeSuggestive OutputMode = "suggestive"
	// OutputModeObservational uses neutral, observational language without prescriptive advice (default)
	OutputModeObservational OutputMode = "observational"
	// OutputModeStatistical shows only raw metrics and numbers without any interpretation
	OutputModeStatistical OutputMode = "statistical"
)

// Report is the top-level canonical output structure.
// It aggregates analysis results from one or more repositories.
type Report struct {
	Meta         ReportMeta    `json:"meta"`
	Repositories []RepoResult  `json:"repositories"`
	Summary      GlobalSummary `json:"summary"` // Aggregated stats across all repos
}

// ReportMeta contains metadata about the execution of the CLI.
type ReportMeta struct {
	GeneratedAt time.Time `json:"generated_at"`
	CLIVersion  string    `json:"cli_version"`
	Command     string    `json:"command"`  // e.g. "run"
	Duration    string    `json:"duration"` // Execution duration
}

// RepoResult contains all metrics and findings for a specific repository.
type RepoResult struct {
	Name      string           `json:"name"` // owner/repo
	URL       string           `json:"url"`
	Analyzers []AnalyzerResult `json:"analyzers"` // Results grouped by analyzer
}

// AnalyzerResult groups output by the specific analyzer that produced it.
type AnalyzerResult struct {
	Name     string    `json:"name"` // e.g. "pr-flow", "security-policy"
	Metrics  []Metric  `json:"metrics,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
}

// Metric represents a quantitative measurement.
// Designed to be easily rendered into CSV or tables.
type Metric struct {
	Key          string  `json:"key"`           // e.g. "avg_time_to_merge_hours"
	Value        float64 `json:"value"`         // Only numeric values for stable metrics
	Unit         string  `json:"unit"`          // e.g. "hours", "count", "percent"
	DisplayValue string  `json:"display_value"` // Human readable: "4.5h"
	Description  string  `json:"description,omitempty"`
}

// Finding represents a qualitative insight or issue detection.
type Finding struct {
	Type             string   `json:"type"` // e.g. "stale_pr", "missing_owner"
	Severity         Severity `json:"severity"`
	Message          string   `json:"message"`
	Location         string   `json:"location,omitempty"` // URL or file path
	Actionable       bool     `json:"actionable"`
	Remediation      string   `json:"remediation,omitempty"`       // Advice on how to fix (suggestive mode)
	Explanation      string   `json:"explanation,omitempty"`       // Why this matters (suggestive/observational modes)
	SuggestedActions []string `json:"suggested_actions,omitempty"` // 1-2 concrete next steps (suggestive mode)
	Observation      string   `json:"observation,omitempty"`       // Neutral observation (observational mode)
}

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// GlobalSummary holds aggregated data useful for multi-repo runs.
type GlobalSummary struct {
	TotalReposAnalyzed int `json:"total_repos_analyzed"`
	IssuesFound        int `json:"issues_found"`

	// Aggregated Metrics
	TotalCommits      int     `json:"total_commits"`
	TotalOpenIssues   int     `json:"total_open_issues"`
	TotalZombieIssues int     `json:"total_zombie_issues"`
	BusFactor1Repos   int     `json:"bus_factor_1_repos"` // Count of repos with BF=1
	ReposAtRisk       int     `json:"repos_at_risk"`      // Count of repos with Health < 50
	AvgHealthScore    float64 `json:"avg_health_score"`
	AvgCISuccessRate  float64 `json:"avg_ci_success_rate"`
	AvgCIRuntime      float64 `json:"avg_ci_runtime"`    // Avg CI runtime in seconds
	AvgPRCycleTime    float64 `json:"avg_pr_cycle_time"` // Avg of avg cycle times
}
