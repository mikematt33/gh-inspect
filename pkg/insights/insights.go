package insights

import (
	"fmt"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

type InsightLevel string

const (
	LevelInfo     InsightLevel = "INFO"
	LevelWarning  InsightLevel = "WARNING"
	LevelCritical InsightLevel = "CRITICAL"
)

type Insight struct {
	Level       InsightLevel
	Category    string
	Description string
	Action      string
}

// GenerateInsights analyzes a single repository report and produces actionable insights
func GenerateInsights(repo models.RepoResult) []Insight {
	var insights []Insight

	// Helper to safely get metric
	getMetric := func(analyzerName, key string) (float64, bool) {
		for _, az := range repo.Analyzers {
			if az.Name == analyzerName {
				for _, m := range az.Metrics {
					if m.Key == key {
						return m.Value, true
					}
				}
			}
		}
		return 0, false
	}

	// 1. Bus Factor Analysis
	busFactor, bfOk := getMetric("activity", "bus_factor")
	activeContributors, acOk := getMetric("activity", "active_contributors")

	if bfOk && acOk && busFactor == 1 && activeContributors > 1 {
		insights = append(insights, Insight{
			Level:       LevelCritical,
			Category:    "Resilience",
			Description: "Bus Factor is 1. A single developer is responsible for >=50% of recent commits.",
			Action:      "Encourage knowledge sharing and pair programming to reduce single usage points of failure.",
		})
	}

	// 2. CI Stability Analysis
	successRate, srOk := getMetric("ci", "success_rate")
	if srOk {
		if successRate < 50.0 {
			insights = append(insights, Insight{
				Level:       LevelCritical,
				Category:    "Velocity",
				Description: fmt.Sprintf("CI active but success rate is dangerously low (%.1f%%).", successRate),
				Action:      "Prioritize fixing flaky tests or broken build steps immediately to unblock the team.",
			})
		} else if successRate < 80.0 {
			insights = append(insights, Insight{
				Level:       LevelWarning,
				Category:    "Velocity",
				Description: fmt.Sprintf("CI success rate is suboptimal (%.1f%%).", successRate),
				Action:      "Investigate common failure patterns to improve developer confidence.",
			})
		}
	}

	// 3. Issue Hygiene (Zombie Issues)
	zombies, zOk := getMetric("issue-hygiene", "zombie_issues")
	if zOk && zombies > 10 {
		insights = append(insights, Insight{
			Level:       LevelWarning,
			Category:    "Maintenance",
			Description: fmt.Sprintf("High number of zombie issues detected (%d).", int(zombies)),
			Action:      "Schedule a 'bug bash' or bulk-close outdated issues to clean up the backlog.",
		})
	}

	// 4. PR Velocity
	cycleTime, ctOk := getMetric("pr-flow", "avg_cycle_time_hours")
	if ctOk && cycleTime > 72.0 { // 3 days
		insights = append(insights, Insight{
			Level:       LevelInfo,
			Category:    "Velocity",
			Description: fmt.Sprintf("Average PR cycle time is high (%.1fh).", cycleTime),
			Action:      "Review PR size and review process. Smaller PRs usually merge faster.",
		})
	}

	return insights
}

// CalculateEngineeringHealthScore produces a 0-100 score based on weighted sub-metrics
func CalculateEngineeringHealthScore(repo models.RepoResult) int {
	score := 100.0

	getMetric := func(analyzerName, key string) (float64, bool) {
		for _, az := range repo.Analyzers {
			if az.Name == analyzerName {
				for _, m := range az.Metrics {
					if m.Key == key {
						return m.Value, true
					}
				}
			}
		}
		return 0, false
	}

	// Deduct for CI instability (Weight: 30)
	successRate, srOk := getMetric("ci", "success_rate")
	if srOk {
		if successRate < 50 {
			score -= 30
		} else if successRate < 90 {
			score -= 15
		}
	}

	// Deduct for Low Bus Factor (Weight: 20)
	busFactor, bfOk := getMetric("activity", "bus_factor")
	activeContributors, acOk := getMetric("activity", "active_contributors")
	if bfOk && acOk {
		if busFactor == 1 && activeContributors > 1 {
			score -= 20
		}
	}

	// Deduct for Zombie Issues (Weight: 15)
	zombies, zOk := getMetric("issue-hygiene", "zombie_issues")
	if zOk {
		if zombies > 50 {
			score -= 15
		} else if zombies > 10 {
			score -= 5
		}
	}

	// Deduct for Missing Key Files (Weight: 5 per file, max 20)
	missingFiles := 0
	// We need to look at findings for repo-health
	for _, az := range repo.Analyzers {
		if az.Name == "repo-health" {
			for _, f := range az.Findings {
				if f.Type == "missing_file" {
					missingFiles++
				}
			}
		}
	}
	if missingFiles > 0 {
		score -= float64(missingFiles * 5)
	}

	// Deduct for stale PRs (Weight: 15)
	stalePRs := 0
	for _, az := range repo.Analyzers {
		if az.Name == "pr-flow" {
			for _, f := range az.Findings {
				if f.Type == "stale_pr" {
					stalePRs++
				}
			}
		}
	}

	if stalePRs > 5 {
		score -= 15
	}

	if score < 0 {
		return 0
	}
	return int(score)
}
