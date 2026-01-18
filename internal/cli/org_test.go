package cli

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestOrgCmd(t *testing.T) {
	// Save originals
	originalPipelineRunner := pipelineRunner
	originalGetOrgRepos := getOrgRepositories
	defer func() {
		pipelineRunner = originalPipelineRunner
		getOrgRepositories = originalGetOrgRepos
	}()

	// Mock repos
	getOrgRepositories = func(orgName string) ([]*github.Repository, error) {
		repo1 := "repo1"
		repo2 := "repo2"
		falseVal := false
		return []*github.Repository{
			{FullName: &repo1, Archived: &falseVal, Fork: &falseVal},
			{FullName: &repo2, Archived: &falseVal, Fork: &falseVal},
		}, nil
	}

	// Mock pipeline
	pipelineRunner = func(opts AnalysisOptions) (*models.Report, error) {
		// Verify expected repos are passed
		expectedLen := 2
		if len(opts.Repos) != expectedLen {
			t.Errorf("Expected %d repos, got %d", expectedLen, len(opts.Repos))
		}
		return &models.Report{
			Summary: models.GlobalSummary{
				TotalReposAnalyzed: 2,
			},
		}, nil
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run command
	orgCmd.SetArgs([]string{"my-org"})
	err := orgCmd.Execute()

	// Restore
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("orgCmd failed: %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check output
	if output == "" {
		t.Errorf("Expected output, got empty string")
	}
}
