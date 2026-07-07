package actions

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestPercentileAndMedian(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}
	if got := median(values); !approx(got, 30) {
		t.Errorf("median = %v, want 30", got)
	}
	if got := percentile(values, 100); !approx(got, 50) {
		t.Errorf("p100 = %v, want 50", got)
	}
	if got := percentile(values, 0); !approx(got, 10) {
		t.Errorf("p0 = %v, want 10", got)
	}
	if got := percentile(nil, 95); got != 0 {
		t.Errorf("p95 of empty = %v, want 0", got)
	}
	if got := percentile([]float64{5}, 95); !approx(got, 5) {
		t.Errorf("single value p95 = %v, want 5", got)
	}
}

func TestFlakiness(t *testing.T) {
	stable := []string{"success", "success", "success", "success"}
	if got := flakiness(stable); got != 0 {
		t.Errorf("stable flakiness = %v, want 0", got)
	}
	alternating := []string{"success", "failure", "success", "failure"}
	if got := flakiness(alternating); !approx(got, 1) {
		t.Errorf("alternating flakiness = %v, want 1", got)
	}
	// Non success/failure conclusions are ignored.
	mixed := []string{"success", "cancelled", "failure"}
	if got := flakiness(mixed); !approx(got, 1) {
		t.Errorf("mixed flakiness = %v, want 1", got)
	}
}

func TestMTBFHours(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	failures := []time.Time{base, base.Add(2 * time.Hour), base.Add(6 * time.Hour)}
	// span 6h over 2 gaps -> 3h
	if got := mtbfHours(failures); !approx(got, 3) {
		t.Errorf("mtbf = %v, want 3", got)
	}
	if got := mtbfHours(failures[:1]); got != 0 {
		t.Errorf("mtbf single = %v, want 0", got)
	}
}

func TestDurationTrend(t *testing.T) {
	// earlier avg 10, later avg 20 -> trend +10
	got := durationTrendSec([]float64{10, 10, 20, 20})
	if !approx(got, 10) {
		t.Errorf("trend = %v, want 10", got)
	}
	if got := durationTrendSec([]float64{1, 2}); got != 0 {
		t.Errorf("trend too few = %v, want 0", got)
	}
}

func TestClassifyRunner(t *testing.T) {
	cases := []struct {
		labels     []string
		hosted     bool
		os         string
		deprecated bool
	}{
		{[]string{"ubuntu-latest"}, true, "linux", false},
		{[]string{"windows-latest"}, true, "windows", false},
		{[]string{"macos-latest"}, true, "macos", false},
		{[]string{"self-hosted", "linux", "x64"}, false, "linux", false},
		{[]string{"my-custom-runner"}, false, "unknown", false},
		{[]string{"ubuntu-18.04"}, true, "linux", true},
	}
	for _, c := range cases {
		rc := classifyRunner(c.labels)
		if rc.hosted != c.hosted || rc.os != c.os || rc.deprecated != c.deprecated {
			t.Errorf("classify(%v) = {hosted:%v os:%s dep:%v}, want {%v %s %v}",
				c.labels, rc.hosted, rc.os, rc.deprecated, c.hosted, c.os, c.deprecated)
		}
	}
}

func TestParseWorkflowTriggers_OnBooleanQuirk(t *testing.T) {
	yaml := `
name: CI
on:
  push:
    branches: [main]
  pull_request:
  schedule:
    - cron: '0 0 * * *'
    - cron: '30 6 * * 1'
jobs:
  build:
    runs-on: ubuntu-latest
`
	trig := parseWorkflowTriggers(yaml)
	want := map[string]bool{"push": true, "pull_request": true, "schedule": true}
	if len(trig.Events) != 3 {
		t.Fatalf("events = %v, want 3 entries", trig.Events)
	}
	for _, e := range trig.Events {
		if !want[e] {
			t.Errorf("unexpected event %q", e)
		}
	}
	if len(trig.Crons) != 2 {
		t.Errorf("crons = %v, want 2", trig.Crons)
	}
}

func TestParseWorkflowTriggers_ListForm(t *testing.T) {
	trig := parseWorkflowTriggers("on: [push, pull_request]\njobs: {}\n")
	if len(trig.Events) != 2 {
		t.Errorf("events = %v, want 2", trig.Events)
	}
}

func TestCronNext(t *testing.T) {
	sched, err := parseCron("0 0 * * *") // daily midnight UTC
	if err != nil {
		t.Fatalf("parseCron: %v", err)
	}
	after := time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC)
	next := sched.Next(after)
	want := time.Date(2024, 3, 11, 0, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("next = %v, want %v", next, want)
	}
}

func TestCronStepAndRange(t *testing.T) {
	sched, err := parseCron("*/15 9-17 * * 1-5")
	if err != nil {
		t.Fatalf("parseCron: %v", err)
	}
	// Monday 2024-03-11 09:00 UTC should match.
	mon := time.Date(2024, 3, 11, 9, 0, 0, 0, time.UTC)
	if !sched.matches(mon) {
		t.Errorf("expected match at %v", mon)
	}
	// Sunday should not match (dow restricted to 1-5).
	sun := time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC)
	if sched.matches(sun) {
		t.Errorf("did not expect match at %v", sun)
	}
}

func TestParseCronInvalid(t *testing.T) {
	if _, err := parseCron("bad"); err == nil {
		t.Error("expected error for invalid cron")
	}
	if _, err := parseCron("60 0 * * *"); err == nil {
		t.Error("expected error for out-of-range minute")
	}
}

func TestHumanizeSeconds(t *testing.T) {
	cases := []struct {
		sec  float64
		want string
	}{
		{0, "0s"},
		{-5, "0s"},
		{45, "45s"},
		{90, "1m30s"},
		{3661, "1h01m"},
		{96948, "1d02h"},
	}
	for _, c := range cases {
		if got := humanizeSeconds(c.sec); got != c.want {
			t.Errorf("humanizeSeconds(%v) = %q, want %q", c.sec, got, c.want)
		}
	}
}

func TestRunnerHostingHint(t *testing.T) {
	if got := runnerHostingHint(models.RunnerUsage{}); got != "" {
		t.Errorf("no data hint = %q, want empty", got)
	}
	if got := runnerHostingHint(models.RunnerUsage{GitHubHosted: 4}); got != "GitHub-hosted runners" {
		t.Errorf("hosted hint = %q", got)
	}
	if got := runnerHostingHint(models.RunnerUsage{SelfHosted: 3}); got != "self-hosted runners" {
		t.Errorf("self hint = %q", got)
	}
	if got := runnerHostingHint(models.RunnerUsage{GitHubHosted: 1, SelfHosted: 3}); got != "75% self-hosted runners" {
		t.Errorf("mixed hint = %q", got)
	}
}

// --- engine test with a mock Fetcher ---

type mockFetcher struct {
	workflows map[string][]*github.Workflow
	runs      []*github.WorkflowRun
	jobs      map[int64][]*github.WorkflowJob
	files     map[string]string
}

func (m *mockFetcher) ListWorkflows(_ context.Context, owner, repo string) ([]*github.Workflow, error) {
	return m.workflows[owner+"/"+repo], nil
}

func (m *mockFetcher) ListWorkflowRuns(_ context.Context, owner, repo, created string, maxRuns int) ([]*github.WorkflowRun, int, error) {
	return m.runs, len(m.runs), nil
}

func (m *mockFetcher) ListWorkflowJobs(_ context.Context, owner, repo string, runID int64) ([]*github.WorkflowJob, error) {
	return m.jobs[runID], nil
}

func (m *mockFetcher) GetFileContent(_ context.Context, owner, repo, path string) (string, error) {
	return m.files[path], nil
}

func ts(t time.Time) *github.Timestamp { return &github.Timestamp{Time: t} }

func TestEngineScanRepo(t *testing.T) {
	now := time.Now()
	wfID := int64(101)
	wf := &github.Workflow{
		ID:      github.Int64(wfID),
		Name:    github.String("CI"),
		Path:    github.String(".github/workflows/ci.yml"),
		State:   github.String("active"),
		HTMLURL: github.String("https://example/ci"),
	}

	mkRun := func(id int64, conclusion string, ageH int, durMin int) *github.WorkflowRun {
		created := now.Add(-time.Duration(ageH) * time.Hour)
		started := created.Add(30 * time.Second)
		updated := started.Add(time.Duration(durMin) * time.Minute)
		return &github.WorkflowRun{
			ID:           github.Int64(id),
			Name:         github.String("CI"),
			WorkflowID:   github.Int64(wfID),
			Event:        github.String("schedule"),
			Status:       github.String("completed"),
			Conclusion:   github.String(conclusion),
			CreatedAt:    ts(created),
			RunStartedAt: ts(started),
			UpdatedAt:    ts(updated),
		}
	}

	mf := &mockFetcher{
		workflows: map[string][]*github.Workflow{"acme/widget": {wf}},
		runs: []*github.WorkflowRun{
			mkRun(1, "success", 100, 5),
			mkRun(2, "failure", 80, 6),
			mkRun(3, "success", 60, 5),
			mkRun(4, "failure", 40, 7),
			mkRun(5, "success", 20, 5),
		},
		jobs: map[int64][]*github.WorkflowJob{
			5: {{
				Name:        github.String("build"),
				Labels:      []string{"ubuntu-latest"},
				StartedAt:   ts(now.Add(-20 * time.Hour)),
				CompletedAt: ts(now.Add(-20*time.Hour + 4*time.Minute)),
			}},
		},
		files: map[string]string{
			".github/workflows/ci.yml": "on:\n  schedule:\n    - cron: '0 0 * * *'\njobs: {}\n",
		},
	}

	eng := NewEngine(mf)
	report, err := eng.ScanRepo(context.Background(), "acme", "widget", Options{Days: 90, SampleJobRuns: 1})
	if err != nil {
		t.Fatalf("ScanRepo: %v", err)
	}

	if report.Totals.WorkflowCount != 1 {
		t.Fatalf("workflow count = %d, want 1", report.Totals.WorkflowCount)
	}
	wfr := report.Workflows[0]
	if wfr.TotalRuns != 5 {
		t.Errorf("total runs = %d, want 5", wfr.TotalRuns)
	}
	if wfr.Success != 3 || wfr.Failure != 2 {
		t.Errorf("success/failure = %d/%d, want 3/2", wfr.Success, wfr.Failure)
	}
	if !approx(wfr.SuccessRate, 60) {
		t.Errorf("success rate = %v, want 60", wfr.SuccessRate)
	}
	if wfr.MTBFHours <= 0 {
		t.Errorf("expected positive MTBF, got %v", wfr.MTBFHours)
	}
	if report.RunnerMix.GitHubHosted != 1 {
		t.Errorf("github-hosted jobs = %d, want 1", report.RunnerMix.GitHubHosted)
	}
	if len(report.Scheduled) != 1 {
		t.Fatalf("scheduled = %d, want 1", len(report.Scheduled))
	}
	if report.Scheduled[0].NextExpected == nil {
		t.Error("expected next scheduled run to be computed")
	}
	if report.Scheduled[0].LastConclusion != "success" {
		t.Errorf("last conclusion = %q, want success", report.Scheduled[0].LastConclusion)
	}
}

type fakeQuota struct{ total int }

func (f *fakeQuota) TotalAvailable() int                    { return f.total }
func (f *fakeQuota) Summaries() []models.TokenStatusSummary { return nil }

func TestEstimatePreflight(t *testing.T) {
	q := &fakeQuota{total: 9000}
	est := EstimatePreflight(PreflightInput{
		Repos:               10,
		AvgWorkflowsPerRepo: 4,
		SampleJobRuns:       0,
		MaxRunsPerRepo:      200,
	}, q)

	// per repo: 1 (list) + 4 (yaml) + ceil(200/100)=2 = 7 -> *10 = 70
	if est.EstimatedAPICalls != 70 {
		t.Errorf("api calls = %d, want 70", est.EstimatedAPICalls)
	}
	if est.EstimatedWorkflows != 40 {
		t.Errorf("workflows = %d, want 40", est.EstimatedWorkflows)
	}
	if !est.Sufficient {
		t.Error("expected sufficient quota")
	}
}

func TestEstimatePreflightInsufficient(t *testing.T) {
	q := &fakeQuota{total: 50}
	est := EstimatePreflight(PreflightInput{Repos: 10, AvgWorkflowsPerRepo: 4, MaxRunsPerRepo: 200}, q)
	if est.Sufficient {
		t.Errorf("expected insufficient quota (have %d, need %d)", est.AvailableQuota, est.EstimatedAPICalls)
	}
}
