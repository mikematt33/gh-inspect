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

func TestExplainScore_CIStability(t *testing.T) {
	tests := []struct {
		name           string
		successRate    float64
		expectedImpact int
		expectTips     bool
	}{
		{
			name:           "Perfect CI (>90%)",
			successRate:    95.0,
			expectedImpact: 0,
			expectTips:     false,
		},
		{
			name:           "Good CI (90%)",
			successRate:    90.0,
			expectedImpact: 0,
			expectTips:     false,
		},
		{
			name:           "Unstable CI (80%)",
			successRate:    80.0,
			expectedImpact: 15,
			expectTips:     true,
		},
		{
			name:           "Broken CI (40%)",
			successRate:    40.0,
			expectedImpact: 30,
			expectTips:     true,
		},
		{
			name:           "Critical CI (10%)",
			successRate:    10.0,
			expectedImpact: 30,
			expectTips:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: tt.successRate},
						},
					},
				},
			}

			components := ExplainScore(repo)

			if len(components) != 1 {
				t.Fatalf("Expected 1 component, got %d", len(components))
			}

			comp := components[0]
			if comp.Category != "CI Stability" {
				t.Errorf("Expected category 'CI Stability', got '%s'", comp.Category)
			}
			if comp.Impact != tt.expectedImpact {
				t.Errorf("Expected impact %d, got %d", tt.expectedImpact, comp.Impact)
			}
			if tt.expectTips && comp.Tips == "" {
				t.Error("Expected tips but got none")
			}
			if !tt.expectTips && comp.Tips != "" {
				t.Error("Expected no tips but got some")
			}
		})
	}
}

func TestExplainScore_BusFactor(t *testing.T) {
	tests := []struct {
		name               string
		busFactor          float64
		activeContributors float64
		expectedImpact     int
		expectTips         bool
	}{
		{
			name:               "Healthy bus factor",
			busFactor:          2,
			activeContributors: 5,
			expectedImpact:     0,
			expectTips:         false,
		},
		{
			name:               "Bus factor 1 with team",
			busFactor:          1,
			activeContributors: 5,
			expectedImpact:     20,
			expectTips:         true,
		},
		{
			name:               "Bus factor 1 solo project",
			busFactor:          1,
			activeContributors: 1,
			expectedImpact:     0,
			expectTips:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name: "activity",
						Metrics: []models.Metric{
							{Key: "bus_factor", Value: tt.busFactor},
							{Key: "active_contributors", Value: tt.activeContributors},
						},
					},
				},
			}

			components := ExplainScore(repo)

			if len(components) != 1 {
				t.Fatalf("Expected 1 component, got %d", len(components))
			}

			comp := components[0]
			if comp.Category != "Team Resilience" {
				t.Errorf("Expected category 'Team Resilience', got '%s'", comp.Category)
			}
			if comp.Impact != tt.expectedImpact {
				t.Errorf("Expected impact %d, got %d", tt.expectedImpact, comp.Impact)
			}
			if tt.expectTips && comp.Tips == "" {
				t.Error("Expected tips but got none")
			}
		})
	}
}

func TestExplainScore_ZombieIssues(t *testing.T) {
	tests := []struct {
		name           string
		zombieCount    float64
		expectedImpact int
	}{
		{
			name:           "No zombies",
			zombieCount:    0,
			expectedImpact: 0,
		},
		{
			name:           "Few zombies",
			zombieCount:    5,
			expectedImpact: 0,
		},
		{
			name:           "Some zombies (15)",
			zombieCount:    15,
			expectedImpact: 5,
		},
		{
			name:           "Many zombies (60)",
			zombieCount:    60,
			expectedImpact: 15,
		},
		{
			name:           "Huge zombie count (200)",
			zombieCount:    200,
			expectedImpact: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name: "issue-hygiene",
						Metrics: []models.Metric{
							{Key: "zombie_issues", Value: tt.zombieCount},
						},
					},
				},
			}

			components := ExplainScore(repo)

			if len(components) != 1 {
				t.Fatalf("Expected 1 component, got %d", len(components))
			}

			comp := components[0]
			if comp.Category != "Issue Hygiene" {
				t.Errorf("Expected category 'Issue Hygiene', got '%s'", comp.Category)
			}
			if comp.Impact != tt.expectedImpact {
				t.Errorf("Expected impact %d, got %d", tt.expectedImpact, comp.Impact)
			}
		})
	}
}

func TestExplainScore_MissingFiles(t *testing.T) {
	tests := []struct {
		name           string
		missingCount   int
		expectedImpact int
	}{
		{
			name:           "No missing files",
			missingCount:   0,
			expectedImpact: 0,
		},
		{
			name:           "1 missing file",
			missingCount:   1,
			expectedImpact: 5,
		},
		{
			name:           "2 missing files",
			missingCount:   2,
			expectedImpact: 10,
		},
		{
			name:           "4 missing files",
			missingCount:   4,
			expectedImpact: 20,
		},
		{
			name:           "10 missing files (capped at 20)",
			missingCount:   10,
			expectedImpact: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var findings []models.Finding
			for i := 0; i < tt.missingCount; i++ {
				findings = append(findings, models.Finding{
					Type:    "missing_file",
					Message: "Missing file",
				})
			}

			repo := models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:     "repo-health",
						Findings: findings,
					},
				},
			}

			components := ExplainScore(repo)

			if tt.missingCount == 0 {
				if len(components) != 0 {
					t.Fatalf("Expected 0 components for no missing files, got %d", len(components))
				}
				return
			}

			if len(components) != 1 {
				t.Fatalf("Expected 1 component, got %d", len(components))
			}

			comp := components[0]
			if comp.Category != "Repository Health" {
				t.Errorf("Expected category 'Repository Health', got '%s'", comp.Category)
			}
			if comp.Impact != tt.expectedImpact {
				t.Errorf("Expected impact %d, got %d", tt.expectedImpact, comp.Impact)
			}
		})
	}
}

func TestExplainScore_StalePRs(t *testing.T) {
	tests := []struct {
		name        string
		staleCount  int
		expectEntry bool
	}{
		{
			name:        "No stale PRs",
			staleCount:  0,
			expectEntry: false,
		},
		{
			name:        "Few stale PRs (5)",
			staleCount:  5,
			expectEntry: false,
		},
		{
			name:        "Many stale PRs (6)",
			staleCount:  6,
			expectEntry: true,
		},
		{
			name:        "Lots of stale PRs (20)",
			staleCount:  20,
			expectEntry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var findings []models.Finding
			for i := 0; i < tt.staleCount; i++ {
				findings = append(findings, models.Finding{
					Type:    "stale_pr",
					Message: "Stale PR",
				})
			}

			repo := models.RepoResult{
				Analyzers: []models.AnalyzerResult{
					{
						Name:     "pr-flow",
						Findings: findings,
					},
				},
			}

			components := ExplainScore(repo)

			if !tt.expectEntry {
				if len(components) != 0 {
					t.Fatalf("Expected 0 components, got %d", len(components))
				}
				return
			}

			if len(components) != 1 {
				t.Fatalf("Expected 1 component, got %d", len(components))
			}

			comp := components[0]
			if comp.Category != "PR Velocity" {
				t.Errorf("Expected category 'PR Velocity', got '%s'", comp.Category)
			}
			if comp.Impact != 15 {
				t.Errorf("Expected impact 15, got %d", comp.Impact)
			}
		})
	}
}

func TestExplainScore_MultipleComponents(t *testing.T) {
	repo := models.RepoResult{
		Analyzers: []models.AnalyzerResult{
			{
				Name: "ci",
				Metrics: []models.Metric{
					{Key: "success_rate", Value: 80.0}, // -15
				},
			},
			{
				Name: "activity",
				Metrics: []models.Metric{
					{Key: "bus_factor", Value: 1},
					{Key: "active_contributors", Value: 5}, // -20
				},
			},
			{
				Name: "issue-hygiene",
				Metrics: []models.Metric{
					{Key: "zombie_issues", Value: 60}, // -15
				},
			},
			{
				Name: "repo-health",
				Findings: []models.Finding{
					{Type: "missing_file", Message: "README.md"},
					{Type: "missing_file", Message: "LICENSE"}, // -10
				},
			},
			{
				Name: "pr-flow",
				Findings: []models.Finding{
					{Type: "stale_pr"},
					{Type: "stale_pr"},
					{Type: "stale_pr"},
					{Type: "stale_pr"},
					{Type: "stale_pr"},
					{Type: "stale_pr"}, // -15
				},
			},
		},
	}

	components := ExplainScore(repo)

	// Should have all 5 components
	if len(components) != 5 {
		t.Fatalf("Expected 5 components, got %d", len(components))
	}

	// Verify total deductions
	totalImpact := 0
	for _, comp := range components {
		totalImpact += comp.Impact
	}

	expectedTotal := 15 + 20 + 15 + 10 + 15
	if totalImpact != expectedTotal {
		t.Errorf("Expected total impact %d, got %d", expectedTotal, totalImpact)
	}

	// Verify categories
	categories := make(map[string]bool)
	for _, comp := range components {
		categories[comp.Category] = true
	}

	expectedCategories := []string{
		"CI Stability",
		"Team Resilience",
		"Issue Hygiene",
		"Repository Health",
		"PR Velocity",
	}

	for _, cat := range expectedCategories {
		if !categories[cat] {
			t.Errorf("Missing expected category: %s", cat)
		}
	}
}

func TestExplainScore_EmptyRepo(t *testing.T) {
	repo := models.RepoResult{
		Analyzers: []models.AnalyzerResult{},
	}

	components := ExplainScore(repo)

	if len(components) != 0 {
		t.Errorf("Expected 0 components for empty repo, got %d", len(components))
	}
}

func TestExplainScore_OnlyMetricsNoFindings(t *testing.T) {
	repo := models.RepoResult{
		Analyzers: []models.AnalyzerResult{
			{
				Name: "ci",
				Metrics: []models.Metric{
					{Key: "success_rate", Value: 95.0},
				},
			},
			{
				Name: "activity",
				Metrics: []models.Metric{
					{Key: "bus_factor", Value: 3},
					{Key: "active_contributors", Value: 5},
				},
			},
			{
				Name: "issue-hygiene",
				Metrics: []models.Metric{
					{Key: "zombie_issues", Value: 2},
				},
			},
		},
	}

	components := ExplainScore(repo)

	// Should have 3 components (CI, Bus Factor, Zombies) all with 0 impact
	if len(components) != 3 {
		t.Fatalf("Expected 3 components, got %d", len(components))
	}

	for _, comp := range components {
		if comp.Impact != 0 {
			t.Errorf("Expected 0 impact for %s, got %d", comp.Category, comp.Impact)
		}
	}
}
