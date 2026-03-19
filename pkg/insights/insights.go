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

	// Deduct for CI instability (Weight: up to 30)
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

	// Deduct for Zombie Issues (Weight: up to 15)
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
		deduction := float64(missingFiles * 5)
		if deduction > 20 {
			deduction = 20
		}
		score -= deduction
	}

	// Deduct for stale PRs (Weight: 15)
	stalePRs, spOk := getMetric("pr-flow", "stale_prs")
	if spOk && stalePRs > 5 {
		score -= 15
	}

	// Deduct for Security vulnerabilities (Weight: up to 20)
	criticalVulns, cvOk := getMetric("security", "dependabot_critical")
	highVulns, hvOk := getMetric("security", "dependabot_high")
	secretAlerts, saOk := getMetric("security", "secret_scanning_alerts")
	if cvOk && criticalVulns > 0 {
		score -= 15
	} else if hvOk && highVulns > 0 {
		score -= 10
	}
	if saOk && secretAlerts > 0 {
		score -= 5
	}

	// Deduct for stale releases (Weight: up to 10)
	daysSinceRelease, dsOk := getMetric("releases", "days_since_last_release")
	releasesInWindow, rwOk := getMetric("releases", "releases_in_window")
	if dsOk && daysSinceRelease > 180 {
		score -= 10
	} else if rwOk && releasesInWindow == 0 && !dsOk {
		// No releases at all (no days_since metric means no releases exist)
		score -= 5
	}

	// Deduct for branch hygiene (Weight: up to 10)
	staleBranches, sbOk := getMetric("branches", "stale_branches")
	totalBranches, tbOk := getMetric("branches", "total_branches")
	if sbOk && staleBranches > 10 {
		score -= 5
	}
	if tbOk && totalBranches > 50 {
		score -= 5
	}

	// Deduct for dependency issues (Weight: up to 10)
	totalDeps, tdOk := getMetric("dependencies", "total_dependencies")
	if tdOk && totalDeps > 100 {
		score -= 5
	}
	// Check for missing lock file or unpinned deps via findings
	for _, az := range repo.Analyzers {
		if az.Name == "dependencies" {
			for _, f := range az.Findings {
				if f.Type == "missing_lock_file" {
					score -= 5
					break
				}
			}
		}
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
	MaxWeight   int    // Maximum possible deduction for this component
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
			MaxWeight:   30,
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
			MaxWeight:   20,
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
			MaxWeight:   15,
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
			tips += fmt.Sprintf(" Missing: %v", missingFileNames[:min(3, len(missingFileNames))])
		}

		components = append(components, ScoreComponent{
			Category:    "Repository Health",
			Description: "Essential documentation files",
			Impact:      impact,
			MaxWeight:   20,
			Current:     fmt.Sprintf("%d missing", missingFiles),
			Target:      "All present",
			Tips:        tips,
		})
	}

	// Stale PRs (Weight: 15)
	stalePRs, spOk := getMetric("pr-flow", "stale_prs")
	if spOk && stalePRs > 5 {
		tips := formatTips(
			"",
			fmt.Sprintf("%.0f pull requests inactive for >14 days.", stalePRs),
			"Review and merge or close old PRs. Long-running PRs often have merge conflicts.",
		)
		components = append(components, ScoreComponent{
			Category:    "PR Velocity",
			Description: "Stale pull requests (>14 days old)",
			Impact:      15,
			MaxWeight:   15,
			Current:     fmt.Sprintf("%.0f stale", stalePRs),
			Target:      "≤5",
			Tips:        tips,
		})
	}

	// Security vulnerabilities (Weight: up to 20)
	criticalVulns, cvOk := getMetric("security", "dependabot_critical")
	highVulns, hvOk := getMetric("security", "dependabot_high")
	secretAlerts, saOk := getMetric("security", "secret_scanning_alerts")
	if cvOk || hvOk || saOk {
		impact := 0
		tips := ""

		if cvOk && criticalVulns > 0 {
			impact += 15
			tips = formatTips(
				"",
				fmt.Sprintf("%.0f critical dependency vulnerabilities detected.", criticalVulns),
				"Update vulnerable dependencies immediately. Critical CVEs are actively exploitable.",
			)
		} else if hvOk && highVulns > 0 {
			impact += 10
			tips = formatTips(
				"",
				fmt.Sprintf("%.0f high-severity dependency vulnerabilities detected.", highVulns),
				"Prioritize updating dependencies with high-severity alerts.",
			)
		}
		if saOk && secretAlerts > 0 {
			impact += 5
			if tips != "" {
				tips += " "
			}
			tips += formatTips(
				"",
				fmt.Sprintf("%.0f leaked secrets detected.", secretAlerts),
				"Rotate leaked credentials and remove from git history.",
			)
		}

		currentParts := []string{}
		if cvOk {
			currentParts = append(currentParts, fmt.Sprintf("%.0f critical", criticalVulns))
		}
		if hvOk {
			currentParts = append(currentParts, fmt.Sprintf("%.0f high", highVulns))
		}
		if saOk && secretAlerts > 0 {
			currentParts = append(currentParts, fmt.Sprintf("%.0f secrets", secretAlerts))
		}
		current := "0 alerts"
		if len(currentParts) > 0 {
			current = fmt.Sprintf("%s", joinParts(currentParts))
		}

		components = append(components, ScoreComponent{
			Category:    "Security",
			Description: "Dependency vulnerabilities and secret scanning",
			Impact:      impact,
			MaxWeight:   20,
			Current:     current,
			Target:      "0 critical/high, 0 secrets",
			Tips:        tips,
		})
	}

	// Releases (Weight: up to 10)
	daysSinceRelease, dsOk := getMetric("releases", "days_since_last_release")
	releasesInWindow, rwOk := getMetric("releases", "releases_in_window")
	if dsOk || rwOk {
		impact := 0
		tips := ""
		current := ""

		if dsOk && daysSinceRelease > 180 {
			impact = 10
			current = fmt.Sprintf("%.0f days since last release", daysSinceRelease)
			tips = formatTips(
				"",
				"No release in over 6 months.",
				"Consider creating a release to ship accumulated changes.",
			)
		} else if rwOk && releasesInWindow == 0 && !dsOk {
			impact = 5
			current = "No releases"
			tips = formatTips(
				"",
				"Repository has no releases.",
				"Use GitHub releases for version tracking and deployment.",
			)
		} else if dsOk {
			current = fmt.Sprintf("%.0f days since last release", daysSinceRelease)
		} else {
			current = fmt.Sprintf("%.0f in window", releasesInWindow)
		}

		components = append(components, ScoreComponent{
			Category:    "Releases",
			Description: "Release frequency and recency",
			Impact:      impact,
			MaxWeight:   10,
			Current:     current,
			Target:      "Release within 180 days",
			Tips:        tips,
		})
	}

	// Branch hygiene (Weight: up to 10)
	staleBranches, sbOk := getMetric("branches", "stale_branches")
	totalBranches, tbOk := getMetric("branches", "total_branches")
	if sbOk || tbOk {
		impact := 0
		tips := ""
		currentParts := []string{}

		if sbOk {
			currentParts = append(currentParts, fmt.Sprintf("%.0f stale", staleBranches))
			if staleBranches > 10 {
				impact += 5
				tips = formatTips(
					"",
					fmt.Sprintf("%.0f stale branches detected.", staleBranches),
					"Delete or merge stale branches to keep the repository clean.",
				)
			}
		}
		if tbOk {
			currentParts = append(currentParts, fmt.Sprintf("%.0f total", totalBranches))
			if totalBranches > 50 {
				impact += 5
				if tips != "" {
					tips += " "
				}
				tips += formatTips(
					"",
					fmt.Sprintf("%.0f total branches is excessive.", totalBranches),
					"Clean up merged branches to reduce clutter.",
				)
			}
		}

		components = append(components, ScoreComponent{
			Category:    "Branch Hygiene",
			Description: "Branch count and staleness",
			Impact:      impact,
			MaxWeight:   10,
			Current:     joinParts(currentParts),
			Target:      "≤50 total, ≤10 stale",
			Tips:        tips,
		})
	}

	// Dependencies (Weight: up to 10)
	totalDeps, tdOk := getMetric("dependencies", "total_dependencies")
	hasLockIssue := false
	for _, az := range repo.Analyzers {
		if az.Name == "dependencies" {
			for _, f := range az.Findings {
				if f.Type == "missing_lock_file" {
					hasLockIssue = true
					break
				}
			}
		}
	}
	if tdOk || hasLockIssue {
		impact := 0
		tips := ""
		current := ""

		if tdOk {
			current = fmt.Sprintf("%.0f dependencies", totalDeps)
			if totalDeps > 100 {
				impact += 5
				tips = formatTips(
					"",
					fmt.Sprintf("High dependency count (%.0f).", totalDeps),
					"Audit and remove unused dependencies to reduce risk.",
				)
			}
		}
		if hasLockIssue {
			impact += 5
			if current == "" {
				current = "No lock file"
			} else {
				current += ", no lock file"
			}
			if tips != "" {
				tips += " "
			}
			tips += formatTips(
				"",
				"Missing lock file for reproducible builds.",
				"Commit a lock file to ensure consistent dependency resolution.",
			)
		}

		components = append(components, ScoreComponent{
			Category:    "Dependencies",
			Description: "Dependency count and lock file presence",
			Impact:      impact,
			MaxWeight:   10,
			Current:     current,
			Target:      "≤100 deps, lock file present",
			Tips:        tips,
		})
	}

	return components
}

// joinParts joins string slices with ", "
func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
