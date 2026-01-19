package report

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/mikematt33/gh-inspect/pkg/insights"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

type Renderer interface {
	Render(report *models.Report, w io.Writer) error
}

func NewRenderer(f Format) Renderer {
	switch f {
	case FormatJSON:
		return &JSONRenderer{}
	case FormatText:
		return &TextRenderer{}
	default:
		return &TextRenderer{}
	}
}

type JSONRenderer struct{}

func (r *JSONRenderer) Render(report *models.Report, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

type TextRenderer struct{}

func (r *TextRenderer) Render(report *models.Report, w io.Writer) error {
	if len(report.Repositories) == 0 {
		_, _ = fmt.Fprintln(w, "No repositories analyzed.")
		return nil
	}

	for _, repo := range report.Repositories {
		_, _ = fmt.Fprintf(w, "\n🔎 REPORT FOR: %s (%s)\n", repo.Name, repo.URL)
		_, _ = fmt.Fprintln(w, "==================================================")

		if len(repo.Analyzers) == 0 {
			_, _ = fmt.Fprintln(w, "No analysis results.")
			continue
		}

		for _, az := range repo.Analyzers {
			_, _ = fmt.Fprintf(w, "\n[ %s ]\n", az.Name)

			// 1. Metrics Table
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

			// 2. Findings List
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
					_, _ = fmt.Fprintf(w, "    %s %s: %s\n", icon, f.Type, f.Message)
				}
			} else {
				_, _ = fmt.Fprintln(w, "  No issues found.")
			}
		}

		// 3. Opinionated Insights & Score
		repoInsights := insights.GenerateInsights(repo)
		engScore := insights.CalculateEngineeringHealthScore(repo)

		_, _ = fmt.Fprintf(w, "\n[ opinionated-insights ]\n")
		_, _ = fmt.Fprintf(w, "  Engineering Health Score: %d/100\n", engScore)

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

	if report.Summary.AvgHealthScore > 0 {
		_, _ = fmt.Fprintf(tw, "Avg Health Score:\t%.1f/100\n", report.Summary.AvgHealthScore)
	}
	if report.Summary.AvgPRCycleTime > 0 {
		_, _ = fmt.Fprintf(tw, "Avg PR Cycle Time:\t%.1fh\n", report.Summary.AvgPRCycleTime)
	}
	if report.Summary.AvgCISuccessRate > 0 || report.Summary.AvgHealthScore > 0 {
		_, _ = fmt.Fprintf(tw, "Avg CI Success Rate:\t%.1f%%\n", report.Summary.AvgCISuccessRate)
	}

	_ = tw.Flush()
	_, _ = fmt.Fprintln(w, "--------------------------------------------------")

	return nil
}
