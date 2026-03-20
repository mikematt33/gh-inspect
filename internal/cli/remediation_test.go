package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
	"github.com/mikematt33/gh-inspect/pkg/remediation"
)

func TestRemediationSetStatusCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remediation.json")
	flagRemediationFile = path
	remediationNote = "working on it"

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runRemediationSetStatus(remediationSetStatusCmd, []string{"abc123", "in-progress"})

	_ = w.Close()
	os.Stdout = oldStdout

	store, err := remediation.Load(path)
	if err != nil {
		t.Fatalf("failed to load remediation store: %v", err)
	}
	if store.Entries["abc123"].Status != remediation.StatusInProgress {
		t.Fatalf("expected in-progress, got %s", store.Entries["abc123"].Status)
	}
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if buf.Len() == 0 {
		t.Fatal("expected command output")
	}
}

func TestRunAnalysisShowsRemediationStatus(t *testing.T) {
	originalPipelineRunner := pipelineRunner
	defer func() { pipelineRunner = originalPipelineRunner }()

	path := filepath.Join(t.TempDir(), "remediation.json")
	flagRemediationFile = path
	store := &remediation.Store{Entries: map[string]remediation.Entry{}}
	finding := models.Finding{Type: "ci_failure", Message: "CI is failing"}
	id := remediation.FindingID("owner/repo", "ci", finding)
	remediation.SetStatus(store, id, remediation.StatusResolved, "fixed")
	if err := remediation.Save(path, store); err != nil {
		t.Fatalf("failed to save remediation store: %v", err)
	}

	pipelineRunner = func(opts AnalysisOptions) (*models.Report, error) {
		return &models.Report{Repositories: []models.RepoResult{{
			Name: "owner/repo",
			URL:  "https://github.com/owner/repo",
			Analyzers: []models.AnalyzerResult{{
				Name:     "ci",
				Findings: []models.Finding{finding},
			}},
		}}}, nil
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	flagFormat = "text"
	flagFail = 0
	flagCompareLast = false
	flagSaveBaseline = false
	flagCompareHistory = 0
	flagBaseline = ""
	flagFailOnRegression = false
	runAnalysis(runCmd, []string{"owner/repo"})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	if !bytes.Contains(buf.Bytes(), []byte("resolved")) {
		t.Fatalf("expected remediation status in output, got %s", buf.String())
	}
}
