package report

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/mikematt33/gh-inspect/pkg/insights"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Format string

const (
	FormatJSON     Format = "json"
	FormatText     Format = "text"
	FormatMarkdown Format = "markdown"
)

// RenderOptions contains options for rendering reports
type RenderOptions struct {
	ShowExplanation bool
	OutputMode      models.OutputMode
	SummaryMode     bool // Only show score + high/medium findings
}

type Renderer interface {
	Render(report *models.Report, w io.Writer) error
	RenderWithOptions(report *models.Report, w io.Writer, opts RenderOptions) error
}

type JSONRenderer struct{}

func (r *JSONRenderer) Render(report *models.Report, w io.Writer) error {
	return r.RenderWithOptions(report, w, RenderOptions{})
}

func (r *JSONRenderer) RenderWithOptions(report *models.Report, w io.Writer, opts RenderOptions) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

type TextRenderer struct{}

func (r *TextRenderer) Render(report *models.Report, w io.Writer) error {
	return r.RenderWithOptions(report, w, RenderOptions{})
}

func (r *TextRenderer) RenderWithOptions(report *models.Report, w io.Writer, opts RenderOptions) error {
	if len(report.Repositories) == 0 {
		_, _ = fmt.Fprintln(w, "No repositories analyzed.")
		return nil
	}

	// Summary mode: compact output with score + high/medium findings only
	if opts.SummaryMode {
		return r.renderSummaryMode(report, w, opts)
	}

	for _, repo := range report.Repositories {
		_, _ = fmt.Fprintf(w, "\n🔎 REPORT FOR: %s (%s)\n", repo.Name, repo.URL)
		_, _ = fmt.Fprintln(w, "==================================================")

		if len(repo.Analyzers) == 0 {
			_, _ = fmt.Fprintln(w, "No analysis results.")
			continue
		}

		// 1. Lead with Score & Insights
		outputMode := opts.OutputMode
		if outputMode == "" {
			outputMode = models.OutputModeObservational // default
		}
		evaluation := insights.EvaluateRepository(repo, outputMode, opts.ShowExplanation)
		engScore := evaluation.Score

		_, _ = fmt.Fprintf(w, "\n[ opinionated-insights ]\n")
		_, _ = fmt.Fprintf(w, "  Engineering Health Score: %d/100\n", engScore)

		// Show score explanation if requested
		if opts.ShowExplanation {
			scoreComponents := evaluation.Components
			if len(scoreComponents) > 0 {
				// Compact weight summary line: "CI: 30/30 · Team: 0/20 · ..."
				_, _ = fmt.Fprintln(w, "")
				_, _ = fmt.Fprintf(w, "  ")
				for i, comp := range scoreComponents {
					earned := comp.MaxWeight - comp.Impact
					if earned < 0 {
						earned = 0
					}
					if i > 0 {
						_, _ = fmt.Fprintf(w, " · ")
					}
					_, _ = fmt.Fprintf(w, "%s: %d/%d", comp.Category, earned, comp.MaxWeight)
				}
				_, _ = fmt.Fprintln(w, "")

				_, _ = fmt.Fprintln(w, "")
				_, _ = fmt.Fprintln(w, "  Score Breakdown:")
				_, _ = fmt.Fprintln(w, "  "+"─────────────────────────────────────────────────────")

				totalImpact := 0
				for _, comp := range scoreComponents {
					totalImpact += comp.Impact

					earned := comp.MaxWeight - comp.Impact
					if earned < 0 {
						earned = 0
					}

					// Show category with earned/max and deduction
					impactStr := ""
					if comp.Impact > 0 {
						impactStr = fmt.Sprintf(" [-%d pts]", comp.Impact)
					} else {
						impactStr = " [✓]"
					}
					_, _ = fmt.Fprintf(w, "  • %s: %d/%d%s\n", comp.Category, earned, comp.MaxWeight, impactStr)
					_, _ = fmt.Fprintf(w, "    Current: %s | Target: %s\n", comp.Current, comp.Target)

					if comp.Tips != "" {
						_, _ = fmt.Fprintf(w, "    💡 %s\n", comp.Tips)
					}
					_, _ = fmt.Fprintln(w, "")
				}

				_, _ = fmt.Fprintf(w, "  Final Score: 100 - %d = %d/100\n", totalImpact, engScore)
			}
		}

		repoInsights := evaluation.Insights
		if len(repoInsights) > 0 {
			_, _ = fmt.Fprintln(w, "")
			for _, ins := range repoInsights {
				icon := "ℹ️"
				switch ins.Level {
				case insights.LevelWarning:
					icon = "⚠️"
				case insights.LevelCritical:
					icon = "🚨"
				}
				_, _ = fmt.Fprintf(w, "  %s %s: %s\n", icon, ins.Category, ins.Description)
				_, _ = fmt.Fprintf(w, "     Action: %s\n", ins.Action)
			}
		} else {
			_, _ = fmt.Fprintln(w, "  No critical insights found.")
		}

		// 2. Then show per-analyzer detail metrics and findings
		for _, az := range repo.Analyzers {
			_, _ = fmt.Fprintf(w, "\n[ %s ]\n", az.Name)

			// Metrics Table
			if len(az.Metrics) > 0 {
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, m := range az.Metrics {
					val := m.DisplayValue
					if val == "" {
						val = fmt.Sprintf("%.2f", m.Value)
					}
					_, _ = fmt.Fprintf(tw, "  %s:\t%s\n", m.Key, val)
				}
				_ = tw.Flush()
				_, _ = fmt.Fprintln(w, "")
			}

			// Findings List (suppressed in statistical mode)
			if opts.OutputMode == models.OutputModeStatistical {
				continue
			}

			if len(az.Findings) > 0 {
				_, _ = fmt.Fprintln(w, "  Findings:")
				for _, f := range az.Findings {
					icon := "ℹ️"
					switch f.Severity {
					case models.SeverityHigh:
						icon = "🚨"
					case models.SeverityMedium:
						icon = "⚠️"
					}
					tracking := ""
					if f.TrackingID != "" {
						tracking = fmt.Sprintf(" [%s|%s]", f.TrackingID, f.RemediationState)
					}
					_, _ = fmt.Fprintf(w, "    %s %s%s: %s\n", icon, f.Type, tracking, f.Message)
					if f.RemediationNote != "" {
						_, _ = fmt.Fprintf(w, "       Note: %s\n", f.RemediationNote)
					}

					// Show explanation if available
					if f.Explanation != "" {
						_, _ = fmt.Fprintf(w, "       Why: %s\n", f.Explanation)
					}

					// Show suggested actions if available
					if len(f.SuggestedActions) > 0 {
						_, _ = fmt.Fprintln(w, "       Actions:")
						for i, action := range f.SuggestedActions {
							_, _ = fmt.Fprintf(w, "       %d. %s\n", i+1, action)
						}
					}
				}
			} else {
				_, _ = fmt.Fprintln(w, "  No issues found.")
			}
		}

		_, _ = fmt.Fprintln(w, "--------------------------------------------------")
	}

	// Render Summary
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "📊 ORGANIZATION SUMMARY")
	_, _ = fmt.Fprintln(w, "==================================================")

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Repositories Analyzed:\t%d\n", report.Summary.TotalReposAnalyzed)
	_, _ = fmt.Fprintf(tw, "Total Commits:\t%d\n", report.Summary.TotalCommits)
	_, _ = fmt.Fprintf(tw, "Total Issues Found:\t%d\n", report.Summary.IssuesFound)
	_, _ = fmt.Fprintf(tw, "Open Issues:\t%d\n", report.Summary.TotalOpenIssues)
	_, _ = fmt.Fprintf(tw, "Zombie Issues:\t%d\n", report.Summary.TotalZombieIssues)
	_, _ = fmt.Fprintf(tw, "Repos At Risk (<50):\t%d\n", report.Summary.ReposAtRisk)
	_, _ = fmt.Fprintf(tw, "Bus Factor 1 Repos:\t%d\n", report.Summary.BusFactor1Repos)
	if report.Summary.RemediationOpen+report.Summary.RemediationInProgress+report.Summary.RemediationResolved+report.Summary.RemediationAccepted+report.Summary.RemediationIgnored > 0 {
		_, _ = fmt.Fprintf(tw, "Remediation Open:\t%d\n", report.Summary.RemediationOpen)
		_, _ = fmt.Fprintf(tw, "Remediation In Progress:\t%d\n", report.Summary.RemediationInProgress)
		_, _ = fmt.Fprintf(tw, "Remediation Resolved:\t%d\n", report.Summary.RemediationResolved)
		_, _ = fmt.Fprintf(tw, "Remediation Accepted:\t%d\n", report.Summary.RemediationAccepted)
		_, _ = fmt.Fprintf(tw, "Remediation Ignored:\t%d\n", report.Summary.RemediationIgnored)
	}

	if report.Summary.AvgHealthScore > 0 {
		_, _ = fmt.Fprintf(tw, "Avg Health Score:\t%.1f/100\n", report.Summary.AvgHealthScore)
	}
	if report.Summary.AvgPRCycleTime > 0 {
		_, _ = fmt.Fprintf(tw, "Avg PR Cycle Time:\t%.1fh\n", report.Summary.AvgPRCycleTime)
	}
	if report.Summary.AvgCISuccessRate > 0 {
		_, _ = fmt.Fprintf(tw, "Avg CI Success Rate:\t%.1f%%\n", report.Summary.AvgCISuccessRate)
	}
	if report.Summary.AvgCIRuntime > 0 {
		_, _ = fmt.Fprintf(tw, "Avg CI Runtime:\t%s\n", (time.Duration(report.Summary.AvgCIRuntime) * time.Second).String())
	}

	_ = tw.Flush()
	_, _ = fmt.Fprintln(w, "--------------------------------------------------")

	return nil
}

// renderSummaryMode outputs a compact view: repo name, score, and high/medium findings only.
func (r *TextRenderer) renderSummaryMode(report *models.Report, w io.Writer, opts RenderOptions) error {
	for _, repo := range report.Repositories {
		engScore := insights.CalculateEngineeringHealthScore(repo)
		scoreEmoji := "🟢"
		switch {
		case engScore < 50:
			scoreEmoji = "🔴"
		case engScore < 75:
			scoreEmoji = "🟠"
		case engScore < 90:
			scoreEmoji = "🟡"
		}

		_, _ = fmt.Fprintf(w, "\n%s %s  Health: %d/100\n", scoreEmoji, repo.Name, engScore)

		// Collect high/medium findings across all analyzers
		hasFindings := false
		for _, az := range repo.Analyzers {
			for _, f := range az.Findings {
				if f.Severity == models.SeverityHigh || f.Severity == models.SeverityMedium {
					if !hasFindings {
						hasFindings = true
					}
					icon := "⚠️"
					if f.Severity == models.SeverityHigh {
						icon = "🚨"
					}
					_, _ = fmt.Fprintf(w, "  %s [%s] %s: %s\n", icon, az.Name, f.Type, f.Message)
				}
			}
		}
		if !hasFindings {
			_, _ = fmt.Fprintln(w, "  No high/medium findings.")
		}
	}

	// Print aggregate if multiple repos
	if len(report.Repositories) > 1 && report.Summary.AvgHealthScore > 0 {
		_, _ = fmt.Fprintf(w, "\n📊 Avg Health: %.0f/100 | %d repos | %d at risk\n",
			report.Summary.AvgHealthScore,
			report.Summary.TotalReposAnalyzed,
			report.Summary.ReposAtRisk)
	}

	return nil
}
