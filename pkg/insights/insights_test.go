package insights

import (
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestCalculateEngineeringHealthScore(t *testing.T) {
	tests := []struct {
		name     string
		repo     models.RepoResult
		expected int
	}{
		{
			name: "Perfect Repo",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:    "ci",
						Metrics: []models.Metric{{Key: "success_rate", Value: 95.0}},
					},
					{
						Name: "activity",
						Metrics: []models.Metric{
							{Key: "bus_factor", Value: 2},
							{Key: "active_contributors", Value: 5},
						},
					},
					{
						Name:    "issue-hygiene",
						Metrics: []models.Metric{{Key: "zombie_issues", Value: 0}},
					},
					{Name: "repo-health", Findings: []models.Finding{}},
					{Name: "pr-flow", Findings: []models.Finding{}},
				},
			},
			expected: 100,
		},
		{
			name: "Unstable CI (-15)",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:    "ci",
						Metrics: []models.Metric{{Key: "success_rate", Value: 80.0}},
					},
				},
			},
			expected: 85,
		},
		{
			name: "Broken CI (-30)",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:    "ci",
						Metrics: []models.Metric{{Key: "success_rate", Value: 40.0}},
					},
				},
			},
			expected: 70,
		},
		{
			name: "Bus Factor 1 Risk (-20)",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name: "activity",
						Metrics: []models.Metric{
							{Key: "bus_factor", Value: 1},
							{Key: "active_contributors", Value: 5},
						},
					},
				},
			},
			expected: 80,
		},
		{
			name: "Huge Zombie Backlog (-15)",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:    "issue-hygiene",
						Metrics: []models.Metric{{Key: "zombie_issues", Value: 100}},
					},
				},
			},
			expected: 85,
		},
		{
			name: "Missing 2 Key Files (-10)",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name: "repo-health",
						Findings: []models.Finding{
							{Type: "missing_file"},
							{Type: "missing_file"},
						},
					},
				},
			},
			expected: 90,
		},
		{
			name: "Everything Wrong (Should be 0 not negative)",
			repo: models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:    "ci",
						Metrics: []models.Metric{{Key: "success_rate", Value: 10.0}}, // -30
					},
					{
						Name: "activity",
						Metrics: []models.Metric{
							{Key: "bus_factor", Value: 1},
							{Key: "active_contributors", Value: 5},
						}, // -20
					},
					{
						Name:    "issue-hygiene",
						Metrics: []models.Metric{{Key: "zombie_issues", Value: 200}}, // -15
					},
					{
						Name: "repo-health",
						Findings: []models.Finding{
							{Type: "missing_file"}, {Type: "missing_file"}, {Type: "missing_file"}, {Type: "missing_file"},
							{Type: "missing_file"}, {Type: "missing_file"},
						}, // -20 (max for files is implicit? code says float64(missing * 5))
						// Logic: 6 files * 5 = 30.
					},
					{
						Name: "pr-flow",
						Findings: []models.Finding{
							{Type: "stale_pr"}, {Type: "stale_pr"}, {Type: "stale_pr"},
							{Type: "stale_pr"}, {Type: "stale_pr"}, {Type: "stale_pr"},
						}, // -15 (>5 stale PRs)
					},
				},
			},
			// Total deducting: 30 + 20 + 15 + 30 + 15 = 110.
			// Result should be 0.
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateEngineeringHealthScore(tt.repo)
			if score != tt.expected {
				t.Errorf("CalculateEngineeringHealthScore() = %v, want %v", score, tt.expected)
			}
		})
	}
}

func TestGenerateInsights(t *testing.T) {
	// Simple test to ensure insights are generated for specific conditions
	repo := models.RepoResult{
		Analyzers: []models.AnalyzerResult{
			{
				Name:    "ci",
				Metrics: []models.Metric{{Key: "success_rate", Value: 40.0}},
			},
			{
				Name:    "pr-flow",
				Metrics: []models.Metric{{Key: "avg_cycle_time_hours", Value: 100.0}},
			},
		},
	}

	insights := GenerateInsights(repo)

	// Expecting:
	// 1. Critical Insight for CI < 50
	// 2. Info Insight for Cycle Time > 72

	if len(insights) != 2 {
		t.Errorf("Expected 2 insights, got %d", len(insights))
	}

	foundCICrit := false
	foundSlowPR := false

	for _, ins := range insights {
		if ins.Category == "Velocity" && ins.Level == LevelCritical {
			foundCICrit = true
		}
		if ins.Category == "Velocity" && ins.Level == LevelInfo {
			foundSlowPR = true
		}
	}

	if !foundCICrit {
		t.Error("Missing Critical CI insight")
	}
	if !foundSlowPR {
		t.Error("Missing Slow PR insight")
	}
}
