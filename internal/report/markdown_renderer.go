package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/mikematt33/gh-inspect/pkg/insights"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

// MarkdownRenderer renders reports in Markdown format suitable for GitHub Actions and PR comments
type MarkdownRenderer struct{}

func (r *MarkdownRenderer) Render(report *models.Report, w io.Writer) error {
	return r.RenderWithOptions(report, w, RenderOptions{})
}

func (r *MarkdownRenderer) RenderWithOptions(report *models.Report, w io.Writer, opts RenderOptions) error {
	if len(report.Repositories) == 0 {
		_, _ = fmt.Fprintln(w, "## 📊 Repository Analysis")
		_, _ = fmt.Fprintln(w, "")
		_, _ = fmt.Fprintln(w, "No repositories analyzed.")
		return nil
	}

	_, _ = fmt.Fprintln(w, "## 📊 Repository Analysis Results")
	_, _ = fmt.Fprintln(w, "")

	for _, repo := range report.Repositories {
		// Calculate score first
		engScore := insights.CalculateEngineeringHealthScore(repo)
		scoreEmoji := getScoreEmoji(engScore)

		_, _ = fmt.Fprintf(w, "### %s %s\n", scoreEmoji, repo.Name)
		_, _ = fmt.Fprintf(w, "**Engineering Health Score: %d/100**\n\n", engScore)

		// Show score breakdown if requested
		if opts.ShowExplanation {
			r.renderScoreBreakdown(repo, engScore, w)
		}

		// Key Metrics Summary
		_, _ = fmt.Fprintln(w, "#### 📈 Key Metrics")
		_, _ = fmt.Fprintln(w, "")
		_, _ = fmt.Fprintln(w, "| Category | Metrics |")
		_, _ = fmt.Fprintln(w, "|----------|---------|")

		for _, az := range repo.Analyzers {
			if len(az.Metrics) > 0 {
				metricsList := []string{}
				for _, m := range az.Metrics {
					val := m.DisplayValue
					if val == "" {
						val = fmt.Sprintf("%.2f", m.Value)
					}
					metricsList = append(metricsList, fmt.Sprintf("**%s:** %s", m.Key, val))
				}
				_, _ = fmt.Fprintf(w, "| %s | %s |\n", az.Name, strings.Join(metricsList, "<br>"))
			}
		}
		_, _ = fmt.Fprintln(w, "")

		// Findings/Issues
		hasFindings := false
		for _, az := range repo.Analyzers {
			if len(az.Findings) > 0 {
				hasFindings = true
				break
			}
		}

		if hasFindings {
			_, _ = fmt.Fprintln(w, "#### 🔍 Findings")
			_, _ = fmt.Fprintln(w, "")

			criticalCount := 0
			warningCount := 0
			infoCount := 0

			for _, az := range repo.Analyzers {
				if len(az.Findings) > 0 {
					_, _ = fmt.Fprintf(w, "<details>\n<summary><b>%s</b> (%d findings)</summary>\n\n", az.Name, len(az.Findings))

					for _, f := range az.Findings {
						icon := "ℹ️"
						switch f.Severity {
						case models.SeverityHigh:
							icon = "🚨"
							criticalCount++
						case models.SeverityMedium:
							icon = "⚠️"
							warningCount++
						default:
							infoCount++
						}
						_, _ = fmt.Fprintf(w, "- %s **%s:** %s\n", icon, f.Type, f.Message)

						// Show explanation if available
						if f.Explanation != "" {
							_, _ = fmt.Fprintf(w, "  - *Why:* %s\n", f.Explanation)
						}

						// Show suggested actions if available
						if len(f.SuggestedActions) > 0 {
							_, _ = fmt.Fprintln(w, "  - *Actions:*")
							for i, action := range f.SuggestedActions {
								_, _ = fmt.Fprintf(w, "    %d. %s\n", i+1, action)
							}
						}
					}

					_, _ = fmt.Fprintln(w, "</details>")
				}
			}

			// Summary badge
			_, _ = fmt.Fprintf(w, "**Summary:** ")
			if criticalCount > 0 {
				_, _ = fmt.Fprintf(w, "🚨 %d critical ", criticalCount)
			}
			if warningCount > 0 {
				_, _ = fmt.Fprintf(w, "⚠️ %d warnings ", warningCount)
			}
			if infoCount > 0 {
				_, _ = fmt.Fprintf(w, "ℹ️ %d info", infoCount)
			}
			_, _ = fmt.Fprintln(w, "")
			_, _ = fmt.Fprintln(w, "")
		}

		// Insights
		repoInsights := insights.GenerateInsights(repo)
		if len(repoInsights) > 0 {
			_, _ = fmt.Fprintln(w, "#### 💡 Recommendations")
			_, _ = fmt.Fprintln(w, "")

			for _, ins := range repoInsights {
				icon := "ℹ️"
				switch ins.Level {
				case insights.LevelWarning:
					icon = "⚠️"
				case insights.LevelCritical:
					icon = "🚨"
				}
				_, _ = fmt.Fprintf(w, "> %s **%s:** %s\n", icon, ins.Category, ins.Description)
				_, _ = fmt.Fprintf(w, "> \n")
				_, _ = fmt.Fprintf(w, "> 💡 **Action:** %s\n\n", ins.Action)
			}
		} else {
			_, _ = fmt.Fprintln(w, "#### ✅ No Critical Insights")
			_, _ = fmt.Fprintln(w, "")
			_, _ = fmt.Fprintln(w, "This repository is in good health!")
			_, _ = fmt.Fprintln(w, "")
		}

		_, _ = fmt.Fprintln(w, "---")
		_, _ = fmt.Fprintln(w, "")
	}

	// Organization Summary
	if len(report.Repositories) > 1 {
		_, _ = fmt.Fprintln(w, "### 📊 Organization Summary")
		_, _ = fmt.Fprintln(w, "")
		_, _ = fmt.Fprintln(w, "| Metric | Value |")
		_, _ = fmt.Fprintln(w, "|--------|-------|")
		_, _ = fmt.Fprintf(w, "| Repositories Analyzed | %d |\n", report.Summary.TotalReposAnalyzed)
		_, _ = fmt.Fprintf(w, "| Total Commits | %d |\n", report.Summary.TotalCommits)
		_, _ = fmt.Fprintf(w, "| Issues Found | %d |\n", report.Summary.IssuesFound)
		_, _ = fmt.Fprintf(w, "| Open Issues | %d |\n", report.Summary.TotalOpenIssues)

		if report.Summary.AvgHealthScore > 0 {
			_, _ = fmt.Fprintf(w, "| Average Health Score | %.1f/100 |\n", report.Summary.AvgHealthScore)
		}
		if report.Summary.AvgPRCycleTime > 0 {
			_, _ = fmt.Fprintf(w, "| Average PR Cycle Time | %.1fh |\n", report.Summary.AvgPRCycleTime)
		}
		if report.Summary.AvgCISuccessRate > 0 {
			_, _ = fmt.Fprintf(w, "| Average CI Success Rate | %.1f%% |\n", report.Summary.AvgCISuccessRate)
		}

		_, _ = fmt.Fprintln(w, "")
	}

	// Footer
	_, _ = fmt.Fprintf(w, "<sub>Generated by [gh-inspect](https://github.com/mikematt33/gh-inspect) at %s</sub>\n",
		report.Meta.GeneratedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func (r *MarkdownRenderer) renderScoreBreakdown(repo models.RepoResult, engScore int, w io.Writer) {
	scoreComponents := insights.ExplainScore(repo)
	if len(scoreComponents) == 0 {
		return
	}

	_, _ = fmt.Fprintln(w, "<details>")
	_, _ = fmt.Fprintln(w, "<summary><b>📊 Score Breakdown</b></summary>")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "| Component | Current | Target | Impact | Tips |")
	_, _ = fmt.Fprintln(w, "|-----------|---------|--------|--------|------|")

	totalImpact := 0
	for _, comp := range scoreComponents {
		totalImpact += comp.Impact

		impactStr := "✓ OK"
		if comp.Impact > 0 {
			impactStr = fmt.Sprintf("-%d pts", comp.Impact)
		}

		tips := comp.Tips
		if tips == "" {
			tips = "-"
		}

		_, _ = fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n",
			comp.Category,
			comp.Current,
			comp.Target,
			impactStr,
			tips)
	}

	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintf(w, "**Final Calculation:** 100 - %d = **%d/100**\n", totalImpact, engScore)
	_, _ = fmt.Fprintln(w, "</details>")
	_, _ = fmt.Fprintln(w, "")
}

func getScoreEmoji(score int) string {
	switch {
	case score >= 90:
		return "🟢"
	case score >= 75:
		return "🟡"
	case score >= 50:
		return "🟠"
	default:
		return "🔴"
	}
}
