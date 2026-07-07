package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestFmtCompute(t *testing.T) {
	cases := []struct {
		minutes float64
		want    string
	}{
		{0, "0 min"},
		{45, "45 min"},
		{600, "600 min (≈10 h)"},
		{17393, "17,393 min (≈290 h)"},
		{80000, "80,000 min (≈56 d)"},
	}
	for _, c := range cases {
		if got := fmtCompute(c.minutes); got != c.want {
			t.Errorf("fmtCompute(%v) = %q, want %q", c.minutes, got, c.want)
		}
	}
}

func TestRunnerLabel(t *testing.T) {
	cases := []struct {
		usage models.RunnerUsage
		want  string
	}{
		{models.RunnerUsage{}, "-"},
		{models.RunnerUsage{GitHubHosted: 5}, "hosted"},
		{models.RunnerUsage{SelfHosted: 5}, "self"},
		{models.RunnerUsage{GitHubHosted: 2, SelfHosted: 3}, "mixed"},
	}
	for _, c := range cases {
		if got := runnerLabel(c.usage); got != c.want {
			t.Errorf("runnerLabel(%+v) = %q, want %q", c.usage, got, c.want)
		}
	}
}

func TestActionsTextRendererOverview(t *testing.T) {
	report := &models.ActionsReport{
		Meta: models.ActionsMeta{Scope: "org", WindowDays: 30},
		Repositories: []models.ActionsRepoReport{
			{Name: "acme/one", Totals: models.RepoActionsTotals{WorkflowCount: 2, TotalRuns: 10, SuccessRate: 90, ComputeMinutes: 120}},
			{Name: "acme/two", Totals: models.RepoActionsTotals{WorkflowCount: 1, TotalRuns: 5, SuccessRate: 80, ComputeMinutes: 30}},
		},
	}
	var buf bytes.Buffer
	if err := (&ActionsTextRenderer{}).Render(report, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "window: last 30 days") {
		t.Errorf("expected scan-scope header, got:\n%s", out)
	}
	if !strings.Contains(out, "OVERVIEW") {
		t.Errorf("expected overview table for multi-repo report, got:\n%s", out)
	}
}
