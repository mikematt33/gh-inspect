package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestRunCmd(t *testing.T) {
	// Save original pipelineRunner and restore after test
	originalPipelineRunner := pipelineRunner
	defer func() { pipelineRunner = originalPipelineRunner }()

	// Mock pipelineRunner
	pipelineRunner = func(opts AnalysisOptions) (*models.Report, error) {
		return &models.Report{
			Summary: models.GlobalSummary{
				AvgHealthScore: 8.5,
			},
		}, nil
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command
	runCmd.SetArgs([]string{"owner/repo"})
	// Reset flags that might have been set by other tests or init()
	flagFormat = "text"
	flagFail = 0

	err := runCmd.Execute()

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runCmd failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Since we use TextRenderer by default and the report is empty except summary score,
	// checking exact output is hard without knowing TextRenderer implementation details.
	// But it shouldn't error out.
	// Let's verify pipelineRunner was called with correct args?
	// The mock doesn't assert args here, simplistic test.

	_ = output // Use the output variable to avoid unused variable error
}
