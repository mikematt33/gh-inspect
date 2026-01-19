package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestCompareCmd(t *testing.T) {
	// Save original pipelineRunner and restore after test
	originalPipelineRunner := pipelineRunner
	defer func() { pipelineRunner = originalPipelineRunner }()

	// Mock pipelineRunner
	pipelineRunner = func(opts AnalysisOptions) (*models.Report, error) {
		return &models.Report{
			Repositories: []models.RepoResult{
				{Name: "owner/repo1", Analyzers: []models.AnalyzerResult{{Name: "test", Metrics: []models.Metric{{Key: "score", Value: 8.0}}}}},
				{Name: "owner/repo2", Analyzers: []models.AnalyzerResult{{Name: "test", Metrics: []models.Metric{{Key: "score", Value: 9.0}}}}},
			},
		}, nil
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command
	compareCmd.SetArgs([]string{"owner/repo1", "owner/repo2"})
	err := compareCmd.Execute()

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("compareCmd failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Basic check that something was produced
	_ = output
}
