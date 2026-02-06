package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mikematt33/gh-inspect/pkg/models"
	"github.com/mikematt33/gh-inspect/pkg/util"
)

// Baseline stores a historical report for comparison
type Baseline struct {
	Timestamp time.Time      `json:"timestamp"`
	Report    *models.Report `json:"report"`
}

// ComparisonResult contains the delta between two reports
type ComparisonResult struct {
	Current  *models.Report    `json:"current"`
	Previous *Baseline         `json:"previous"`
	Deltas   []RepositoryDelta `json:"deltas"`
	Summary  ComparisonSummary `json:"summary"`
}

// RepositoryDelta contains changes for a single repository
type RepositoryDelta struct {
	RepoName    string         `json:"repo_name"`
	MetricDiff  []MetricChange `json:"metric_diff"`
	FindingDiff FindingChange  `json:"finding_diff"`
}

// MetricChange represents the change in a metric
type MetricChange struct {
	Key          string  `json:"key"`
	Previous     float64 `json:"previous"`
	Current      float64 `json:"current"`
	Delta        float64 `json:"delta"`
	PercentDelta float64 `json:"percent_delta"`
	Improved     bool    `json:"improved"` // Whether this change is positive
}

// FindingChange tracks changes in findings
type FindingChange struct {
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Unchanged int `json:"unchanged"`
}

// ComparisonSummary provides high-level comparison stats
type ComparisonSummary struct {
	HasRegression        bool    `json:"has_regression"`
	HealthScoreDelta     float64 `json:"health_score_delta"`
	CISuccessRateDelta   float64 `json:"ci_success_rate_delta"`
	PRCycleTimeDelta     float64 `json:"pr_cycle_time_delta"`
	ZombieIssueDelta     int     `json:"zombie_issue_delta"`
	TotalImprovedMetrics int     `json:"total_improved_metrics"`
	TotalDegradedMetrics int     `json:"total_degraded_metrics"`
}

// Save persists a report as a baseline
func Save(report *models.Report, path string) error {
	baseline := Baseline{
		Timestamp: time.Now(),
		Report:    report,
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create baseline directory: %w", err)
	}

	// Write to file
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baseline: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write baseline: %w", err)
	}

	return nil
}

// Load reads a baseline from disk
func Load(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline: %w", err)
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("failed to unmarshal baseline: %w", err)
	}

	return &baseline, nil
}

// Compare generates a comparison between current and previous reports
func Compare(current *models.Report, previous *Baseline) *ComparisonResult {
	if current == nil || previous == nil || previous.Report == nil {
		return nil
	}

	result := &ComparisonResult{
		Current:  current,
		Previous: previous,
		Deltas:   make([]RepositoryDelta, 0),
	}

	// Build map of previous repos for easy lookup
	prevRepos := make(map[string]*models.RepoResult)
	for i := range previous.Report.Repositories {
		repo := &previous.Report.Repositories[i]
		prevRepos[repo.Name] = repo
	}

	// Compare each current repo with previous
	for i := range current.Repositories {
		currRepo := &current.Repositories[i]
		prevRepo, exists := prevRepos[currRepo.Name]
		if !exists {
			continue // New repo, skip comparison
		}

		delta := compareRepository(currRepo, prevRepo)
		result.Deltas = append(result.Deltas, delta)
	}

	// Generate summary
	result.Summary = generateSummary(current, previous.Report, result.Deltas)

	return result
}

// compareRepository compares two repository results
func compareRepository(current, previous *models.RepoResult) RepositoryDelta {
	delta := RepositoryDelta{
		RepoName:   current.Name,
		MetricDiff: make([]MetricChange, 0),
	}

	// Build map of previous metrics
	prevMetrics := make(map[string]float64)
	for _, analyzer := range previous.Analyzers {
		for _, metric := range analyzer.Metrics {
			key := analyzer.Name + "." + metric.Key
			prevMetrics[key] = metric.Value
		}
	}

	// Compare current metrics
	for _, analyzer := range current.Analyzers {
		for _, metric := range analyzer.Metrics {
			key := analyzer.Name + "." + metric.Key
			prevValue, exists := prevMetrics[key]
			if !exists {
				continue // New metric
			}

			if metric.Value != prevValue {
				change := MetricChange{
					Key:      key,
					Previous: prevValue,
					Current:  metric.Value,
					Delta:    metric.Value - prevValue,
				}

				if prevValue != 0 {
					change.PercentDelta = (metric.Value - prevValue) / prevValue * 100
				}

				// Determine if change is improvement (heuristic based on metric name)
				change.Improved = isImprovement(metric.Key, change.Delta)

				delta.MetricDiff = append(delta.MetricDiff, change)
			}
		}
	}

	// Compare findings counts
	currFindings := countFindings(current)
	prevFindings := countFindings(previous)

	delta.FindingDiff = FindingChange{
		Added:     util.Max(0, currFindings-prevFindings),
		Removed:   util.Max(0, prevFindings-currFindings),
		Unchanged: util.Min(currFindings, prevFindings),
	}

	return delta
}

// isImprovement determines if a metric change is positive
func isImprovement(key string, delta float64) bool {
	// Metrics where higher is better
	improvesWithIncrease := []string{
		"health_score",
		"success_rate",
		"merge_ratio",
		"label_coverage",
		"assignee_coverage",
		"branch_protection_enabled",
	}

	// Metrics where lower is better
	improvesWithDecrease := []string{
		"cycle_time",
		"failure_count",
		"stale_",
		"zombie_",
		"avg_issue_lifetime",
		"avg_first_response_time",
		"self_merge_rate",
	}

	for _, pattern := range improvesWithIncrease {
		if strings.Contains(key, pattern) {
			return delta > 0
		}
	}

	for _, pattern := range improvesWithDecrease {
		if strings.Contains(key, pattern) {
			return delta < 0
		}
	}

	// Default: assume higher is better
	return delta > 0
}

// generateSummary creates a high-level comparison summary
func generateSummary(current, previous *models.Report, deltas []RepositoryDelta) ComparisonSummary {
	summary := ComparisonSummary{
		HealthScoreDelta:   current.Summary.AvgHealthScore - previous.Summary.AvgHealthScore,
		CISuccessRateDelta: current.Summary.AvgCISuccessRate - previous.Summary.AvgCISuccessRate,
		PRCycleTimeDelta:   current.Summary.AvgPRCycleTime - previous.Summary.AvgPRCycleTime,
		ZombieIssueDelta:   current.Summary.TotalZombieIssues - previous.Summary.TotalZombieIssues,
	}

	// Count improved/degraded metrics
	for _, delta := range deltas {
		for _, change := range delta.MetricDiff {
			if change.Improved {
				summary.TotalImprovedMetrics++
			} else {
				summary.TotalDegradedMetrics++
			}
		}
	}

	// Determine if there's a regression (conservative definition)
	summary.HasRegression = summary.HealthScoreDelta < -5 ||
		summary.CISuccessRateDelta < -10 ||
		summary.ZombieIssueDelta > 5 ||
		summary.TotalDegradedMetrics > summary.TotalImprovedMetrics*2

	return summary
}

// countFindings returns total findings count for a repo
func countFindings(repo *models.RepoResult) int {
	total := 0
	for _, analyzer := range repo.Analyzers {
		total += len(analyzer.Findings)
	}
	return total
}

// GetDefaultBaselinePath returns the default path for baseline storage
func GetDefaultBaselinePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gh-inspect/baseline.json"
	}
	return filepath.Join(home, ".gh-inspect", "baseline.json")
}
