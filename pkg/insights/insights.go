package insights

import (
	"fmt"

	"github.com/mikematt33/gh-inspect/pkg/models"
	"github.com/mikematt33/gh-inspect/pkg/util"
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
	Description string // Main message (mode-aware)
	Action      string // Suggestive mode: prescriptive advice
	Observation string // Observational mode: neutral facts
	StatValue   string // Statistical mode: just the number
}

// GenerateInsights analyzes a single repository report and produces actionable insights
// The output format is controlled by the outputMode parameter
func GenerateInsights(repo models.RepoResult, outputMode models.OutputMode) []Insight {
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

	// Helper to format messages based on output mode
	formatMessage := func(statValue string, observation string, action string) string {
		switch outputMode {
		case models.OutputModeStatistical:
			return statValue
		case models.OutputModeObservational:
			return observation
		case models.OutputModeSuggestive:
			return observation + " " + action
		default:
			return observation
		}
	}

	// 1. Bus Factor Analysis
	busFactor, bfOk := getMetric("activity", "bus_factor")
	activeContributors, acOk := getMetric("activity", "active_contributors")

	if bfOk && acOk && busFactor == 1 && activeContributors > 1 {
		insights = append(insights, Insight{
			Level:    LevelCritical,
			Category: "Resilience",
			Description: formatMessage(
				"Bus Factor: 1",
				"Bus factor is 1. A single developer is responsible for >=50% of recent commits.",
				"Encourage knowledge sharing and pair programming to reduce single points of failure.",
			),
			Action:      "Encourage knowledge sharing and pair programming to reduce single points of failure.",
			Observation: "Bus factor is 1. A single developer is responsible for >=50% of recent commits.",
			StatValue:   "Bus Factor: 1",
		})
	}

	// 2. CI Stability Analysis
	successRate, srOk := getMetric("ci", "success_rate")
	if srOk {
		if successRate < 50.0 {
			insights = append(insights, Insight{
				Level:    LevelCritical,
				Category: "Velocity",
				Description: formatMessage(
					fmt.Sprintf("CI Success Rate: %.1f%%", successRate),
					fmt.Sprintf("CI is active but success rate is %.1f%%.", successRate),
					"Prioritize fixing flaky tests or broken build steps immediately to unblock the team.",
				),
				Action:      "Prioritize fixing flaky tests or broken build steps immediately to unblock the team.",
				Observation: fmt.Sprintf("CI is active but success rate is %.1f%%.", successRate),
				StatValue:   fmt.Sprintf("CI Success Rate: %.1f%%", successRate),
			})
		} else if successRate < 80.0 {
			insights = append(insights, Insight{
				Level:    LevelWarning,
				Category: "Velocity",
				Description: formatMessage(
					fmt.Sprintf("CI Success Rate: %.1f%%", successRate),
					fmt.Sprintf("CI success rate is %.1f%%.", successRate),
					"Investigate common failure patterns to improve developer confidence.",
				),
				Action:      "Investigate common failure patterns to improve developer confidence.",
				Observation: fmt.Sprintf("CI success rate is %.1f%%.", successRate),
				StatValue:   fmt.Sprintf("CI Success Rate: %.1f%%", successRate),
			})
		}
	}

	// 3. Issue Hygiene (Zombie Issues)
	zombies, zOk := getMetric("issue-hygiene", "zombie_issues")
	if zOk && zombies > 10 {
		insights = append(insights, Insight{
			Level:    LevelWarning,
			Category: "Maintenance",
			Description: formatMessage(
				fmt.Sprintf("Zombie Issues: %d", int(zombies)),
				fmt.Sprintf("%d zombie issues detected (inactive >90 days).", int(zombies)),
				"Schedule a 'bug bash' or bulk-close outdated issues to clean up the backlog.",
			),
			Action:      "Schedule a 'bug bash' or bulk-close outdated issues to clean up the backlog.",
			Observation: fmt.Sprintf("%d zombie issues detected (inactive >90 days).", int(zombies)),
			StatValue:   fmt.Sprintf("Zombie Issues: %d", int(zombies)),
		})
	}

	// 4. PR Velocity
	cycleTime, ctOk := getMetric("pr-flow", "avg_cycle_time_hours")
	if ctOk && cycleTime > 72.0 { // 3 days
		insights = append(insights, Insight{
			Level:    LevelInfo,
			Category: "Velocity",
			Description: formatMessage(
				fmt.Sprintf("Avg PR Cycle Time: %.1fh", cycleTime),
				fmt.Sprintf("Average PR cycle time is %.1fh.", cycleTime),
				"Review PR size and review process. Smaller PRs usually merge faster.",
			),
			Action:      "Review PR size and review process. Smaller PRs usually merge faster.",
			Observation: fmt.Sprintf("Average PR cycle time is %.1fh.", cycleTime),
			StatValue:   fmt.Sprintf("Avg PR Cycle Time: %.1fh", cycleTime),
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

// ScoreComponent represents a component of the health score calculation
type ScoreComponent struct {
	Category    string
	Description string
	Impact      int    // Points deducted
	Current     string // Current value
	Target      string // Target/ideal value
	Tips        string // Mode-aware improvement information
}

// ExplainScore returns detailed breakdown of how the health score was calculated
// The output format is controlled by the outputMode parameter
func ExplainScore(repo models.RepoResult, outputMode models.OutputMode) []ScoreComponent {
	var components []ScoreComponent

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

	// Helper to format tips based on mode
	formatTips := func(statisticalMsg, observationalMsg, suggestiveMsg string) string {
		switch outputMode {
		case models.OutputModeStatistical:
			return statisticalMsg
		case models.OutputModeObservational:
			return observationalMsg
		case models.OutputModeSuggestive:
			return suggestiveMsg
		default:
			return observationalMsg
		}
	}

	// CI Stability (Weight: 30)
	successRate, srOk := getMetric("ci", "success_rate")
	if srOk {
		impact := 0
		tips := ""

		if successRate < 50 {
			impact = 30
			tips = formatTips(
				"",
				"CI success rate below 50% correlates with reduced team productivity.",
				"Fix failing builds immediately. CI below 50% blocks team productivity.",
			)
		} else if successRate < 90 {
			impact = 15
			tips = formatTips(
				"",
				"CI success rate between 50-90% suggests intermittent build issues.",
				"Investigate flaky tests and common failure patterns.",
			)
		}

		components = append(components, ScoreComponent{
			Category:    "CI Stability",
			Description: "Continuous Integration success rate",
			Impact:      impact,
			Current:     fmt.Sprintf("%.1f%%", successRate),
			Target:      "≥90%",
			Tips:        tips,
		})
	}

	// Bus Factor (Weight: 20)
	busFactor, bfOk := getMetric("activity", "bus_factor")
	activeContributors, acOk := getMetric("activity", "active_contributors")
	if bfOk && acOk {
		impact := 0
		tips := ""

		if busFactor == 1 && activeContributors > 1 {
			impact = 20
			tips = formatTips(
				"",
				"Single contributor accounts for >50% of commits.",
				"One person is doing >50% of commits. Encourage pair programming and knowledge sharing.",
			)
		}

		components = append(components, ScoreComponent{
			Category:    "Team Resilience",
			Description: "Bus factor (key person dependency)",
			Impact:      impact,
			Current:     fmt.Sprintf("%.0f", busFactor),
			Target:      "≥2",
			Tips:        tips,
		})
	}

	// Zombie Issues (Weight: 15)
	zombies, zOk := getMetric("issue-hygiene", "zombie_issues")
	if zOk {
		impact := 0
		tips := ""

		if zombies > 50 {
			impact = 15
			tips = formatTips(
				"",
				"High volume of inactive issues (>90 days without updates).",
				"High zombie count. Schedule a bug bash to close stale issues.",
			)
		} else if zombies > 10 {
			impact = 5
			tips = formatTips(
				"",
				"Moderate number of inactive issues detected.",
				"Some stale issues detected. Review and close outdated items.",
			)
		}

		components = append(components, ScoreComponent{
			Category:    "Issue Hygiene",
			Description: "Stale/zombie issues (>90 days inactive)",
			Impact:      impact,
			Current:     fmt.Sprintf("%.0f", zombies),
			Target:      "≤10",
			Tips:        tips,
		})
	}

	// Repository Health Files (Weight: 5 per file, max 20)
	missingFiles := 0
	missingFileNames := []string{}
	for _, az := range repo.Analyzers {
		if az.Name == "repo-health" {
			for _, f := range az.Findings {
				if f.Type == "missing_file" {
					missingFiles++
					// Extract file name from message if possible
					missingFileNames = append(missingFileNames, f.Message)
				}
			}
		}
	}

	if missingFiles > 0 {
		impact := missingFiles * 5
		if impact > 20 {
			impact = 20
		}

		tips := formatTips(
			"",
			fmt.Sprintf("Missing %d documentation files.", missingFiles),
			"Add missing documentation files to improve project health.",
		)
		if len(missingFileNames) > 0 && outputMode != models.OutputModeStatistical {
			tips += fmt.Sprintf(" Missing: %v", missingFileNames[:util.Min(3, len(missingFileNames))])
		}

		components = append(components, ScoreComponent{
			Category:    "Repository Health",
			Description: "Essential documentation files",
			Impact:      impact,
			Current:     fmt.Sprintf("%d missing", missingFiles),
			Target:      "All present",
			Tips:        tips,
		})
	}

	// Stale PRs (Weight: 15)
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
		tips := formatTips(
			"",
			fmt.Sprintf("%d pull requests inactive for >14 days.", stalePRs),
			"Review and merge or close old PRs. Long-running PRs often have merge conflicts.",
		)
		components = append(components, ScoreComponent{
			Category:    "PR Velocity",
			Description: "Stale pull requests (>14 days old)",
			Impact:      15,
			Current:     fmt.Sprintf("%d stale", stalePRs),
			Target:      "≤5",
			Tips:        tips,
		})
	}

	// Calculate final score display
	totalDeductions := 0
	for _, c := range components {
		totalDeductions += c.Impact
	}

	return components
}
