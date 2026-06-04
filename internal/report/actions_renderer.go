package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

// ActionsRenderer renders a GitHub Actions analytics report.
type ActionsRenderer interface {
	Render(report *models.ActionsReport, w io.Writer) error
}

// NewActionsRenderer returns a renderer for the requested format
// ("json", "markdown", or "text").
func NewActionsRenderer(format string) ActionsRenderer {
	switch format {
	case "json":
		return &ActionsJSONRenderer{}
	case "markdown":
		return &ActionsMarkdownRenderer{}
	default:
		return &ActionsTextRenderer{}
	}
}

// ActionsJSONRenderer emits the report as indented JSON.
type ActionsJSONRenderer struct{}

func (r *ActionsJSONRenderer) Render(report *models.ActionsReport, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// ActionsTextRenderer emits a human-readable terminal report.
type ActionsTextRenderer struct{}

func (r *ActionsTextRenderer) Render(report *models.ActionsReport, w io.Writer) error {
	for _, repo := range report.Repositories {
		_, _ = fmt.Fprintf(w, "\n⚙️  ACTIONS REPORT: %s\n", repo.Name)
		_, _ = fmt.Fprintln(w, "==================================================")
		t := repo.Totals
		_, _ = fmt.Fprintf(w, "Workflows: %d   Runs: %d   Success rate: %.1f%%   Avg duration: %s   Compute: %.0f min\n",
			t.WorkflowCount, t.TotalRuns, t.SuccessRate, fmtDur(t.AvgDurationSec), t.ComputeMinutes)

		if mix := repo.RunnerMix; mix.GitHubHosted+mix.SelfHosted > 0 {
			_, _ = fmt.Fprintf(w, "Runners: %d GitHub-hosted, %d self-hosted", mix.GitHubHosted, mix.SelfHosted)
			if len(mix.ByOS) > 0 {
				_, _ = fmt.Fprintf(w, " (by OS: %s)", joinCounts(mix.ByOS))
			}
			_, _ = fmt.Fprintln(w)
			if len(mix.Deprecated) > 0 {
				_, _ = fmt.Fprintf(w, "⚠️  Deprecated runner labels in use: %s\n", strings.Join(mix.Deprecated, ", "))
			}
		}

		if len(repo.Workflows) > 0 {
			_, _ = fmt.Fprintln(w, "\nPer-workflow:")
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "  WORKFLOW\tRUNS\tSUCCESS\tAVG\tP95\tMTBF(h)\tFLAKY")
			for _, wf := range repo.Workflows {
				flaky := ""
				if wf.Flaky {
					flaky = "yes"
				}
				_, _ = fmt.Fprintf(tw, "  %s\t%d\t%.0f%%\t%s\t%s\t%s\t%s\n",
					truncate(wf.Name, 28), wf.TotalRuns, wf.SuccessRate,
					fmtDur(wf.AvgDurationSec), fmtDur(wf.P95DurationSec), fmtMTBF(wf.MTBFHours), flaky)
			}
			_ = tw.Flush()
		}

		// Slowest jobs (if job-level data was collected).
		for _, wf := range repo.Workflows {
			if len(wf.SlowestJobs) == 0 {
				continue
			}
			_, _ = fmt.Fprintf(w, "\n  Slowest jobs in %q:\n", wf.Name)
			for _, j := range wf.SlowestJobs {
				_, _ = fmt.Fprintf(w, "    - %s: avg %s (max %s, n=%d)\n",
					j.Name, fmtDur(j.AvgDurationSec), fmtDur(j.MaxDurationSec), j.Samples)
			}
		}

		// Scheduled workflows.
		if len(repo.Scheduled) > 0 {
			_, _ = fmt.Fprintln(w, "\nScheduled workflows:")
			for _, s := range repo.Scheduled {
				last := "never"
				if s.LastRun != nil {
					last = fmt.Sprintf("%s (%s)", s.LastRun.Format("2006-01-02 15:04"), valueOr(s.LastConclusion, "?"))
				}
				next := "unknown"
				if s.NextExpected != nil {
					next = s.NextExpected.Format("2006-01-02 15:04 MST")
				}
				_, _ = fmt.Fprintf(w, "  - %s: last %s, next %s [%s]\n", s.Workflow, last, next, strings.Join(s.CronExprs, "; "))
			}
		}

		// Findings.
		var findings []models.Finding
		for _, wf := range repo.Workflows {
			findings = append(findings, wf.Findings...)
		}
		if len(findings) > 0 {
			_, _ = fmt.Fprintln(w, "\nFindings:")
			for _, f := range findings {
				_, _ = fmt.Fprintf(w, "  %s %s\n", severityIcon(f.Severity), f.Message)
			}
		}
	}

	// Org roll-up.
	if report.Summary.ReposScanned > 1 {
		s := report.Summary
		_, _ = fmt.Fprintf(w, "\n📊 ORG ROLL-UP (%d repos)\n", s.ReposScanned)
		_, _ = fmt.Fprintln(w, "==================================================")
		_, _ = fmt.Fprintf(w, "Total workflows: %d   Total runs: %d   Overall success: %.1f%%\n",
			s.TotalWorkflows, s.TotalRuns, s.OverallSuccessRate)
		_, _ = fmt.Fprintf(w, "Compute minutes: %.0f GitHub-hosted, %.0f self-hosted\n",
			s.ComputeMinutesHosted, s.ComputeMinutesSelf)
		if len(s.FailureHotspots) > 0 {
			_, _ = fmt.Fprintln(w, "\nFailure hotspots:")
			for _, h := range s.FailureHotspots {
				_, _ = fmt.Fprintf(w, "  - %s / %s: %d failures of %d runs (%.0f%%)\n",
					h.Repo, h.Workflow, h.Failures, h.Runs, h.FailRate)
			}
		}
	}

	// Token status.
	if len(report.TokenStatus) > 0 {
		_, _ = fmt.Fprintln(w, "\nCredential status:")
		for _, ts := range report.TokenStatus {
			state := "ok"
			if ts.Invalid {
				state = "INVALID"
			} else if ts.Exhausted {
				state = "EXHAUSTED"
			}
			_, _ = fmt.Fprintf(w, "  - %s (%s): %d/%d remaining [%s]\n", ts.Name, ts.Kind, ts.Remaining, ts.Limit, state)
		}
	}
	return nil
}

// ActionsMarkdownRenderer emits a Markdown report suitable for PRs or step
// summaries.
type ActionsMarkdownRenderer struct{}

func (r *ActionsMarkdownRenderer) Render(report *models.ActionsReport, w io.Writer) error {
	_, _ = fmt.Fprintln(w, "# GitHub Actions Analytics")
	_, _ = fmt.Fprintf(w, "\n_Generated %s · window %d days_\n", report.Meta.GeneratedAt.Format(time.RFC3339), report.Meta.WindowDays)

	for _, repo := range report.Repositories {
		t := repo.Totals
		_, _ = fmt.Fprintf(w, "\n## %s\n\n", repo.Name)
		_, _ = fmt.Fprintf(w, "- Workflows: **%d**\n- Runs: **%d**\n- Success rate: **%.1f%%**\n- Avg duration: **%s**\n- Compute: **%.0f min**\n",
			t.WorkflowCount, t.TotalRuns, t.SuccessRate, fmtDur(t.AvgDurationSec), t.ComputeMinutes)
		mix := repo.RunnerMix
		if mix.GitHubHosted+mix.SelfHosted > 0 {
			_, _ = fmt.Fprintf(w, "- Runners: %d GitHub-hosted / %d self-hosted\n", mix.GitHubHosted, mix.SelfHosted)
		}

		if len(repo.Workflows) > 0 {
			_, _ = fmt.Fprintln(w, "\n| Workflow | Runs | Success | Avg | P95 | Flaky |")
			_, _ = fmt.Fprintln(w, "|---|---:|---:|---:|---:|:--:|")
			for _, wf := range repo.Workflows {
				flaky := ""
				if wf.Flaky {
					flaky = "⚠️"
				}
				_, _ = fmt.Fprintf(w, "| %s | %d | %.0f%% | %s | %s | %s |\n",
					wf.Name, wf.TotalRuns, wf.SuccessRate, fmtDur(wf.AvgDurationSec), fmtDur(wf.P95DurationSec), flaky)
			}
		}

		var findings []models.Finding
		for _, wf := range repo.Workflows {
			findings = append(findings, wf.Findings...)
		}
		if len(findings) > 0 {
			_, _ = fmt.Fprintln(w, "\n### Findings")
			for _, f := range findings {
				_, _ = fmt.Fprintf(w, "- %s **%s** — %s\n", severityIcon(f.Severity), f.Severity, f.Message)
			}
		}
	}

	if report.Summary.ReposScanned > 1 {
		s := report.Summary
		_, _ = fmt.Fprintf(w, "\n## Org roll-up (%d repos)\n\n", s.ReposScanned)
		_, _ = fmt.Fprintf(w, "- Total workflows: **%d**\n- Total runs: **%d**\n- Overall success: **%.1f%%**\n- Compute: %.0f min hosted / %.0f min self-hosted\n",
			s.TotalWorkflows, s.TotalRuns, s.OverallSuccessRate, s.ComputeMinutesHosted, s.ComputeMinutesSelf)
		if len(s.FailureHotspots) > 0 {
			_, _ = fmt.Fprintln(w, "\n### Failure hotspots")
			_, _ = fmt.Fprintln(w, "\n| Repo | Workflow | Failures | Runs | Fail rate |")
			_, _ = fmt.Fprintln(w, "|---|---|---:|---:|---:|")
			for _, h := range s.FailureHotspots {
				_, _ = fmt.Fprintf(w, "| %s | %s | %d | %d | %.0f%% |\n", h.Repo, h.Workflow, h.Failures, h.Runs, h.FailRate)
			}
		}
	}
	return nil
}

// RenderPreflight prints a pre-flight estimate summary to w.
func RenderPreflight(est *models.PreflightEstimate, w io.Writer) {
	_, _ = fmt.Fprintln(w, "Estimated scope:")
	_, _ = fmt.Fprintf(w, "  Repos:              %d\n", est.Repos)
	_, _ = fmt.Fprintf(w, "  Workflows (est.):   ~%d\n", est.EstimatedWorkflows)
	_, _ = fmt.Fprintf(w, "  API calls (est.):   ~%s\n", humanizeInt(est.EstimatedAPICalls))
	_, _ = fmt.Fprintln(w, "\nAvailable quota:")
	for _, s := range est.Sources {
		label := s.Name
		if s.Invalid {
			label += " (invalid)"
		} else if s.Exhausted {
			label += " (exhausted)"
		}
		_, _ = fmt.Fprintf(w, "  %-20s %s remaining\n", label+":", humanizeInt(s.Remaining))
	}
	_, _ = fmt.Fprintf(w, "  %-20s %s remaining\n", "Total:", humanizeInt(est.AvailableQuota))
	_, _ = fmt.Fprintln(w)
	if est.Sufficient {
		_, _ = fmt.Fprintln(w, "✓ Sufficient quota to proceed.")
	} else {
		_, _ = fmt.Fprintln(w, "✗ Insufficient quota to complete this scan.")
	}
}

// --- formatting helpers ---

func fmtDur(sec float64) string {
	if sec <= 0 {
		return "-"
	}
	d := time.Duration(sec * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func fmtMTBF(hours float64) string {
	if hours <= 0 {
		return "-"
	}
	if hours < 24 {
		return fmt.Sprintf("%.1f", hours)
	}
	return fmt.Sprintf("%.1fd", hours/24)
}

func severityIcon(s models.Severity) string {
	switch s {
	case models.SeverityHigh:
		return "🔴"
	case models.SeverityMedium:
		return "🟠"
	case models.SeverityLow:
		return "🟡"
	default:
		return "ℹ️"
	}
}

func joinCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func humanizeInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	neg := false
	if n < 0 {
		neg = true
		s = s[1:]
	}
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
