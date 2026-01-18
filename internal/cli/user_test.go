package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestUserCmd(t *testing.T) {
	// Save originals
	originalPipelineRunner := pipelineRunner
	originalGetUserRepos := getUserRepositories
	defer func() {
		pipelineRunner = originalPipelineRunner
		getUserRepositories = originalGetUserRepos
	}()

	// Mock repos
	getUserRepositories = func(username string) ([]*github.Repository, error) {
		repo1 := "repo1"
		falseVal := false
		return []*github.Repository{
			{FullName: &repo1, Archived: &falseVal, Fork: &falseVal},
		}, nil
	}

	// Mock pipeline
	pipelineRunner = func(opts AnalysisOptions) (*models.Report, error) {
		return &models.Report{
			Summary: models.GlobalSummary{
				TotalReposAnalyzed: 1,
				AvgHealthScore:     90,
			},
		}, nil
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command
	userCmd.SetArgs([]string{"my-user"})
	err := userCmd.Execute()

	// Restore
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("userCmd failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if output == "" {
		t.Errorf("Expected output, got empty string")
	}
}
