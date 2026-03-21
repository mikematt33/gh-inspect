package remediation

import (
	"path/filepath"
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestApplyReportAssignsIDsAndStatuses(t *testing.T) {
	report := &models.Report{
		Repositories: []models.RepoResult{{
			Name: "owner/repo",
			Analyzers: []models.AnalyzerResult{{
				Name:     "ci",
				Findings: []models.Finding{{Type: "ci_failure", Message: "CI is failing"}},
			}},
		}},
	}

	store := &Store{Entries: map[string]Entry{}}
	ApplyReport(report, store)

	finding := report.Repositories[0].Analyzers[0].Findings[0]
	if finding.TrackingID == "" {
		t.Fatal("expected tracking id to be populated")
	}
	if finding.RemediationState != string(StatusOpen) {
		t.Fatalf("expected default status open, got %s", finding.RemediationState)
	}
	if report.Summary.RemediationOpen != 1 {
		t.Fatalf("expected remediation open count 1, got %d", report.Summary.RemediationOpen)
	}
}

func TestSaveLoadAndSetStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remediation.json")
	store := &Store{Entries: map[string]Entry{}}
	SetStatus(store, "abc123", StatusResolved, "fixed in #42")
	if err := Save(path, store); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	entry := loaded.Entries["abc123"]
	if entry.Status != StatusResolved {
		t.Fatalf("expected resolved status, got %s", entry.Status)
	}
	if entry.Note != "fixed in #42" {
		t.Fatalf("unexpected note %q", entry.Note)
	}
}
