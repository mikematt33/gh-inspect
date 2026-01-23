package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func createTestReport(healthScore, ciSuccessRate, prCycleTime float64, zombieIssues int) *models.Report {
	return &models.Report{
		Summary: models.GlobalSummary{
			AvgHealthScore:    healthScore,
			AvgCISuccessRate:  ciSuccessRate,
			AvgPRCycleTime:    prCycleTime,
			TotalZombieIssues: zombieIssues,
		},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo1",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: ciSuccessRate},
						},
						Findings: []models.Finding{
							{Severity: models.SeverityMedium, Message: "Test finding"},
						},
					},
					{
						Name: "prflow",
						Metrics: []models.Metric{
							{Key: "cycle_time", Value: prCycleTime},
						},
					},
				},
			},
		},
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.json")

	// Create test report
	report := createTestReport(85.0, 95.0, 2.5, 5)

	// Save baseline
	err := Save(report, baselinePath)
	if err != nil {
		t.Fatalf("Failed to save baseline: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		t.Fatal("Baseline file was not created")
	}

	// Load baseline
	loaded, err := Load(baselinePath)
	if err != nil {
		t.Fatalf("Failed to load baseline: %v", err)
	}

	// Verify data
	if loaded.Report.Summary.AvgHealthScore != 85.0 {
		t.Errorf("Expected health score 85.0, got %f", loaded.Report.Summary.AvgHealthScore)
	}
	if loaded.Report.Summary.AvgCISuccessRate != 95.0 {
		t.Errorf("Expected CI success rate 95.0, got %f", loaded.Report.Summary.AvgCISuccessRate)
	}
	if loaded.Report.Summary.AvgPRCycleTime != 2.5 {
		t.Errorf("Expected PR cycle time 2.5, got %f", loaded.Report.Summary.AvgPRCycleTime)
	}
	if loaded.Report.Summary.TotalZombieIssues != 5 {
		t.Errorf("Expected zombie issues 5, got %d", loaded.Report.Summary.TotalZombieIssues)
	}

	// Verify timestamp
	if loaded.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/baseline.json")
	if err == nil {
		t.Error("Expected error when loading nonexistent baseline")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(baselinePath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	_, err = Load(baselinePath)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}
}

func TestCompare(t *testing.T) {
	// Create previous baseline
	previousReport := createTestReport(80.0, 90.0, 3.0, 10)
	previous := &Baseline{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Report:    previousReport,
	}

	// Create current report (improved)
	currentReport := createTestReport(85.0, 95.0, 2.5, 8)

	// Compare
	result := Compare(currentReport, previous)

	// Verify comparison result
	if result == nil {
		t.Fatal("Expected non-nil comparison result")
	}
	if result.Current != currentReport {
		t.Error("Current report mismatch")
	}
	if result.Previous != previous {
		t.Error("Previous baseline mismatch")
	}

	// Verify summary
	summary := result.Summary
	if summary.HealthScoreDelta != 5.0 {
		t.Errorf("Expected health score delta 5.0, got %f", summary.HealthScoreDelta)
	}
	if summary.CISuccessRateDelta != 5.0 {
		t.Errorf("Expected CI success rate delta 5.0, got %f", summary.CISuccessRateDelta)
	}
	if summary.PRCycleTimeDelta != -0.5 {
		t.Errorf("Expected PR cycle time delta -0.5, got %f", summary.PRCycleTimeDelta)
	}
	if summary.ZombieIssueDelta != -2 {
		t.Errorf("Expected zombie issue delta -2, got %d", summary.ZombieIssueDelta)
	}
	if summary.HasRegression {
		t.Error("Expected no regression for improved metrics")
	}
}

func TestCompareWithRegression(t *testing.T) {
	// Create previous baseline
	previousReport := createTestReport(85.0, 95.0, 2.5, 5)
	previous := &Baseline{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Report:    previousReport,
	}

	// Create current report (degraded)
	currentReport := createTestReport(75.0, 80.0, 5.0, 15)

	// Compare
	result := Compare(currentReport, previous)

	// Verify regression is detected
	summary := result.Summary
	if summary.HealthScoreDelta != -10.0 {
		t.Errorf("Expected health score delta -10.0, got %f", summary.HealthScoreDelta)
	}
	if summary.CISuccessRateDelta != -15.0 {
		t.Errorf("Expected CI success rate delta -15.0, got %f", summary.CISuccessRateDelta)
	}
	if summary.ZombieIssueDelta != 10 {
		t.Errorf("Expected zombie issue delta 10, got %d", summary.ZombieIssueDelta)
	}
	if !summary.HasRegression {
		t.Error("Expected regression to be detected")
	}
}

func TestCompareMetricChanges(t *testing.T) {
	// Create previous report
	previousReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: 90.0},
							{Key: "failure_count", Value: 10.0},
						},
					},
				},
			},
		},
	}
	previous := &Baseline{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Report:    previousReport,
	}

	// Create current report with changes
	currentReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: 95.0}, // Improved
							{Key: "failure_count", Value: 5.0},  // Improved (lower is better)
						},
					},
				},
			},
		},
	}

	// Compare
	result := Compare(currentReport, previous)

	// Verify deltas
	if len(result.Deltas) != 1 {
		t.Fatalf("Expected 1 repository delta, got %d", len(result.Deltas))
	}

	delta := result.Deltas[0]
	if delta.RepoName != "test/repo" {
		t.Errorf("Expected repo name 'test/repo', got '%s'", delta.RepoName)
	}

	// Check metric changes
	if len(delta.MetricDiff) != 2 {
		t.Fatalf("Expected 2 metric changes, got %d", len(delta.MetricDiff))
	}

	// Verify success_rate improvement
	var successRateChange *MetricChange
	var failureCountChange *MetricChange
	for i := range delta.MetricDiff {
		if delta.MetricDiff[i].Key == "ci.success_rate" {
			successRateChange = &delta.MetricDiff[i]
		}
		if delta.MetricDiff[i].Key == "ci.failure_count" {
			failureCountChange = &delta.MetricDiff[i]
		}
	}

	if successRateChange == nil {
		t.Fatal("Expected success_rate metric change")
	}
	if !successRateChange.Improved {
		t.Error("Expected success_rate to be marked as improved")
	}
	if successRateChange.Delta != 5.0 {
		t.Errorf("Expected delta 5.0, got %f", successRateChange.Delta)
	}

	if failureCountChange == nil {
		t.Fatal("Expected failure_count metric change")
	}
	if !failureCountChange.Improved {
		t.Error("Expected failure_count to be marked as improved (lower is better)")
	}
	if failureCountChange.Delta != -5.0 {
		t.Errorf("Expected delta -5.0, got %f", failureCountChange.Delta)
	}
}

func TestCompareFindingsChange(t *testing.T) {
	// Previous report with 3 findings
	previousReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "security",
						Findings: []models.Finding{
							{Severity: models.SeverityMedium, Message: "Finding 1"},
							{Severity: models.SeverityMedium, Message: "Finding 2"},
							{Severity: models.SeverityHigh, Message: "Finding 3"},
						},
					},
				},
			},
		},
	}
	previous := &Baseline{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Report:    previousReport,
	}

	// Current report with 5 findings
	currentReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "security",
						Findings: []models.Finding{
							{Severity: models.SeverityMedium, Message: "Finding 1"},
							{Severity: models.SeverityMedium, Message: "Finding 2"},
							{Severity: models.SeverityHigh, Message: "Finding 3"},
							{Severity: models.SeverityMedium, Message: "Finding 4"},
							{Severity: models.SeverityHigh, Message: "Finding 5"},
						},
					},
				},
			},
		},
	}

	// Compare
	result := Compare(currentReport, previous)

	// Verify finding changes
	if len(result.Deltas) != 1 {
		t.Fatalf("Expected 1 repository delta, got %d", len(result.Deltas))
	}

	findingDiff := result.Deltas[0].FindingDiff
	if findingDiff.Added != 2 {
		t.Errorf("Expected 2 added findings, got %d", findingDiff.Added)
	}
	if findingDiff.Removed != 0 {
		t.Errorf("Expected 0 removed findings, got %d", findingDiff.Removed)
	}
	if findingDiff.Unchanged != 3 {
		t.Errorf("Expected 3 unchanged findings, got %d", findingDiff.Unchanged)
	}
}

func TestCompareNewRepository(t *testing.T) {
	// Previous report with one repo
	previousReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo1",
				Analyzers: []models.AnalyzerResult{
					{Name: "ci", Metrics: []models.Metric{{Key: "success_rate", Value: 90.0}}},
				},
			},
		},
	}
	previous := &Baseline{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Report:    previousReport,
	}

	// Current report with two repos (one new)
	currentReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo1",
				Analyzers: []models.AnalyzerResult{
					{Name: "ci", Metrics: []models.Metric{{Key: "success_rate", Value: 90.0}}},
				},
			},
			{
				Name: "test/repo2", // New repo
				Analyzers: []models.AnalyzerResult{
					{Name: "ci", Metrics: []models.Metric{{Key: "success_rate", Value: 85.0}}},
				},
			},
		},
	}

	// Compare
	result := Compare(currentReport, previous)

	// Should only have delta for repo1, repo2 is skipped as new
	if len(result.Deltas) != 1 {
		t.Errorf("Expected 1 repository delta (new repo skipped), got %d", len(result.Deltas))
	}
	if len(result.Deltas) > 0 && result.Deltas[0].RepoName != "test/repo1" {
		t.Errorf("Expected delta for 'test/repo1', got '%s'", result.Deltas[0].RepoName)
	}
}

func TestIsImprovement(t *testing.T) {
	tests := []struct {
		key      string
		delta    float64
		expected bool
	}{
		// Higher is better metrics
		{"health_score", 5.0, true},
		{"health_score", -5.0, false},
		{"success_rate", 10.0, true},
		{"success_rate", -10.0, false},
		{"merge_ratio", 2.0, true},
		
		// Lower is better metrics
		{"cycle_time", -1.0, true},
		{"cycle_time", 1.0, false},
		{"failure_count", -5.0, true},
		{"failure_count", 5.0, false},
		{"stale_issues", -2.0, true},
		{"stale_issues", 2.0, false},
		{"zombie_issues", -3.0, true},
		{"zombie_issues", 3.0, false},
		
		// Default (higher is better)
		{"unknown_metric", 5.0, true},
		{"unknown_metric", -5.0, false},
	}

	for _, tt := range tests {
		result := isImprovement(tt.key, tt.delta)
		if result != tt.expected {
			t.Errorf("isImprovement(%s, %f) = %v, expected %v", tt.key, tt.delta, result, tt.expected)
		}
	}
}

func TestCountFindings(t *testing.T) {
	repo := &models.RepoResult{
		Name: "test/repo",
		Analyzers: []models.AnalyzerResult{
			{
				Name: "analyzer1",
				Findings: []models.Finding{
					{Severity: models.SeverityMedium, Message: "Finding 1"},
					{Severity: models.SeverityHigh, Message: "Finding 2"},
				},
			},
			{
				Name: "analyzer2",
				Findings: []models.Finding{
					{Severity: models.SeverityInfo, Message: "Finding 3"},
				},
			},
		},
	}

	count := countFindings(repo)
	if count != 3 {
		t.Errorf("Expected 3 findings, got %d", count)
	}
}

func TestGetDefaultBaselinePath(t *testing.T) {
	path := GetDefaultBaselinePath()
	if path == "" {
		t.Error("Expected non-empty default baseline path")
	}
	
	// Should contain .gh-inspect directory
	if !filepath.IsAbs(path) && path != ".gh-inspect/baseline.json" {
		t.Errorf("Expected absolute path or '.gh-inspect/baseline.json', got '%s'", path)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Path with nested directory that doesn't exist
	baselinePath := filepath.Join(tmpDir, "nested", "dir", "baseline.json")

	report := createTestReport(80.0, 90.0, 3.0, 5)

	err := Save(report, baselinePath)
	if err != nil {
		t.Fatalf("Failed to save baseline: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		t.Fatal("Baseline file was not created in nested directory")
	}
}

func TestCompareWithZeroMetrics(t *testing.T) {
	// Test percent delta calculation when previous value is 0
	previousReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: 0.0},
						},
					},
				},
			},
		},
	}
	previous := &Baseline{
		Timestamp: time.Now().Add(-24 * time.Hour),
		Report:    previousReport,
	}

	currentReport := &models.Report{
		Summary: models.GlobalSummary{},
		Repositories: []models.RepoResult{
			{
				Name: "test/repo",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: 50.0},
						},
					},
				},
			},
		},
	}

	result := Compare(currentReport, previous)

	// Should handle gracefully without panic
	if len(result.Deltas) != 1 {
		t.Fatalf("Expected 1 delta, got %d", len(result.Deltas))
	}

	if len(result.Deltas[0].MetricDiff) != 1 {
		t.Fatalf("Expected 1 metric change, got %d", len(result.Deltas[0].MetricDiff))
	}

	change := result.Deltas[0].MetricDiff[0]
	if change.Delta != 50.0 {
		t.Errorf("Expected delta 50.0, got %f", change.Delta)
	}
	// PercentDelta should be 0 when dividing by 0
	if change.PercentDelta != 0.0 {
		t.Errorf("Expected percent delta 0.0 when previous is 0, got %f", change.PercentDelta)
	}
}

func TestRegressionDetectionThresholds(t *testing.T) {
	tests := []struct {
		name                string
		healthScoreDelta    float64
		ciSuccessRateDelta  float64
		zombieIssueDelta    int
		improvedMetrics     int
		degradedMetrics     int
		expectRegression    bool
	}{
		{
			name:             "No regression - small changes",
			healthScoreDelta: -2.0,
			ciSuccessRateDelta: -3.0,
			zombieIssueDelta: 1,
			improvedMetrics: 5,
			degradedMetrics: 2,
			expectRegression: false,
		},
		{
			name:             "Regression - health score drop > 5",
			healthScoreDelta: -6.0,
			ciSuccessRateDelta: 0.0,
			zombieIssueDelta: 0,
			improvedMetrics: 10,
			degradedMetrics: 1,
			expectRegression: true,
		},
		{
			name:             "Regression - CI success rate drop > 10",
			healthScoreDelta: 0.0,
			ciSuccessRateDelta: -11.0,
			zombieIssueDelta: 0,
			improvedMetrics: 10,
			degradedMetrics: 1,
			expectRegression: true,
		},
		{
			name:             "Regression - zombie issues increase > 5",
			healthScoreDelta: 0.0,
			ciSuccessRateDelta: 0.0,
			zombieIssueDelta: 6,
			improvedMetrics: 10,
			degradedMetrics: 1,
			expectRegression: true,
		},
		{
			name:             "Regression - degraded > 2x improved",
			healthScoreDelta: 0.0,
			ciSuccessRateDelta: 0.0,
			zombieIssueDelta: 0,
			improvedMetrics: 2,
			degradedMetrics: 5, // 5 > 2*2
			expectRegression: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &models.Report{
				Summary: models.GlobalSummary{
					AvgHealthScore:    80.0,
					AvgCISuccessRate:  90.0,
					TotalZombieIssues: 5,
				},
			}
			
			current := &models.Report{
				Summary: models.GlobalSummary{
					AvgHealthScore:    80.0 + tt.healthScoreDelta,
					AvgCISuccessRate:  90.0 + tt.ciSuccessRateDelta,
					TotalZombieIssues: 5 + tt.zombieIssueDelta,
				},
			}

			// Create fake deltas with the specified improved/degraded counts
			deltas := []RepositoryDelta{}
			for i := 0; i < tt.improvedMetrics; i++ {
				deltas = append(deltas, RepositoryDelta{
					MetricDiff: []MetricChange{{Improved: true}},
				})
			}
			for i := 0; i < tt.degradedMetrics; i++ {
				deltas = append(deltas, RepositoryDelta{
					MetricDiff: []MetricChange{{Improved: false}},
				})
			}

			summary := generateSummary(current, previous, deltas)
			
			if summary.HasRegression != tt.expectRegression {
				t.Errorf("Expected regression=%v, got %v", tt.expectRegression, summary.HasRegression)
			}
		})
	}
}

func TestBaselineJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	baselinePath := filepath.Join(tmpDir, "baseline.json")

	report := createTestReport(85.0, 95.0, 2.5, 5)
	err := Save(report, baselinePath)
	if err != nil {
		t.Fatalf("Failed to save baseline: %v", err)
	}

	// Read raw JSON
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("Failed to read baseline file: %v", err)
	}

	// Verify it's valid JSON
	var rawJSON map[string]interface{}
	err = json.Unmarshal(data, &rawJSON)
	if err != nil {
		t.Fatalf("Baseline is not valid JSON: %v", err)
	}

	// Verify expected fields exist
	if _, ok := rawJSON["timestamp"]; !ok {
		t.Error("Expected 'timestamp' field in baseline JSON")
	}
	if _, ok := rawJSON["report"]; !ok {
		t.Error("Expected 'report' field in baseline JSON")
	}
}
