package prflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
)

// MockClient implements analysis.Client for testing
type MockClient struct {
	PullRequests   []*github.PullRequest
	Reviews        map[int][]*github.PullRequestReview
	SinglePR       map[int]*github.PullRequest
	WorkflowRuns   *github.WorkflowRuns
	Repositories   []*github.Repository
	Commits        []*github.RepositoryCommit
	Issues         []*github.Issue
	CombinedStatus *github.CombinedStatus
	Content        *github.RepositoryContent // simplified
}

func (m *MockClient) GetPullRequests(ctx context.Context, owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	var filtered []*github.PullRequest
	for _, pr := range m.PullRequests {
		// Mock filtering by State
		if opts.State == "all" || opts.State == pr.GetState() {
			filtered = append(filtered, pr)
		}
	}
	return filtered, nil
}

func (m *MockClient) GetReviews(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.PullRequestReview, error) {
	return m.Reviews[number], nil
}

func (m *MockClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	return m.SinglePR[number], nil
}

// Unused methods stubbed
func (m *MockClient) ListCommitsSince(ctx context.Context, owner, repo string, since time.Time) ([]*github.RepositoryCommit, error) {
	return m.Commits, nil
}
func (m *MockClient) GetRateLimit(ctx context.Context) (*github.Rate, error) {
	return &github.Rate{}, nil
}
func (m *MockClient) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	return nil, nil
}
func (m *MockClient) GetContent(ctx context.Context, owner, repo, path string) (*github.RepositoryContent, []*github.RepositoryContent, error) {
	return nil, nil, nil
}
func (m *MockClient) GetCombinedStatus(ctx context.Context, owner, repo, ref string) (*github.CombinedStatus, error) {
	return m.CombinedStatus, nil
}
func (m *MockClient) GetIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, error) {
	return m.Issues, nil
}
func (m *MockClient) GetIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, error) {
	return nil, nil
}
func (m *MockClient) GetWorkflowRuns(ctx context.Context, owner, repo string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error) {
	return m.WorkflowRuns, nil, nil
}
func (m *MockClient) ListRepositories(ctx context.Context, org string, opts *github.RepositoryListByOrgOptions) ([]*github.Repository, error) {
	return m.Repositories, nil
}
func (m *MockClient) GetUnderlyingClient() *github.Client {
	return nil
}
func (m *MockClient) GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*github.Tree, error) {
	return nil, nil
}

func TestAnalyzer_Analyze(t *testing.T) {
	now := time.Now()
	twoDaysAgo := now.Add(-48 * time.Hour)
	fourDaysAgo := now.Add(-96 * time.Hour)
	tenDaysAgo := now.Add(-240 * time.Hour)

	closedPR := &github.PullRequest{
		Number:    github.Int(1),
		State:     github.String("closed"),
		CreatedAt: &github.Timestamp{Time: fourDaysAgo},
		ClosedAt:  &github.Timestamp{Time: twoDaysAgo},
		MergedAt:  &github.Timestamp{Time: twoDaysAgo}, // Cycle time ~48h
		UpdatedAt: &github.Timestamp{Time: twoDaysAgo},
		User:      &github.User{Login: github.String("dev1")},
		HTMLURL:   github.String("http://github.com/owner/repo/pull/1"),
	}

	stalePR := &github.PullRequest{
		Number:    github.Int(2),
		State:     github.String("open"),
		CreatedAt: &github.Timestamp{Time: tenDaysAgo},
		UpdatedAt: &github.Timestamp{Time: tenDaysAgo}, // No updates since creation
		User:      &github.User{Login: github.String("dev2")},
		Draft:     github.Bool(false),
		HTMLURL:   github.String("http://github.com/owner/repo/pull/2"),
	}

	// Giant PR
	giantPRDetail := &github.PullRequest{
		Number:    github.Int(3),
		State:     github.String("closed"),
		CreatedAt: &github.Timestamp{Time: now},
		ClosedAt:  &github.Timestamp{Time: now},
		MergedAt:  &github.Timestamp{Time: now},
		UpdatedAt: &github.Timestamp{Time: now},
		Additions: github.Int(2000), // > 1000 threshold
		Deletions: github.Int(500),
		HTMLURL:   github.String("http://github.com/owner/repo/pull/3"),
	}
	giantPRListItem := &github.PullRequest{
		Number:    github.Int(3),
		State:     github.String("closed"),
		CreatedAt: &github.Timestamp{Time: now},
		ClosedAt:  &github.Timestamp{Time: now},
		MergedAt:  &github.Timestamp{Time: now},
		UpdatedAt: &github.Timestamp{Time: now},
		HTMLURL:   github.String("http://github.com/owner/repo/pull/3"),
	}

	mockClient := &MockClient{
		PullRequests: []*github.PullRequest{closedPR, stalePR, giantPRListItem},
		SinglePR: map[int]*github.PullRequest{
			1: closedPR,
			2: stalePR,
			3: giantPRDetail,
		},
		Reviews: map[int][]*github.PullRequestReview{},
	}

	analyzer := New(7) // 7 days stale threshold

	ctx := context.Background()
	repo := analysis.TargetRepository{Owner: "test", Name: "repo"}
	cfg := analysis.Config{
		Since: tenDaysAgo.Add(-24 * time.Hour),
	}

	result, err := analyzer.Analyze(ctx, mockClient, repo, cfg)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// 1. Check Cycle Time
	// PR 1: 48 hours.
	// PR 3: 0 hours.
	// Avg: 24h.
	foundCycleTime := false
	for _, m := range result.Metrics {
		if m.Key == "avg_cycle_time_hours" {
			foundCycleTime = true
			if m.Value < 23 || m.Value > 25 {
				t.Errorf("Expected cycle time ~24h, got %v", m.Value)
			}
		}
	}
	if !foundCycleTime {
		t.Error("Metric avg_cycle_time_hours not found")
	}

	// 2. Check Stale PR Finding
	foundStale := false
	for _, f := range result.Findings {
		if f.Type == "stale_pr" && strings.Contains(f.Location, "pull/2") {
			foundStale = true
		}
	}
	if !foundStale {
		t.Error("Expected stale_pr finding for PR #2")
	}

	// 3. Check Giant PR Finding
	foundGiant := false
	for _, f := range result.Findings {
		// "Large PR detected: #3"
		if f.Type == "giant_pr" && strings.Contains(f.Message, "#3") {
			foundGiant = true
		}
	}
	if !foundGiant {
		// Log findings if possible
		t.Logf("Findings found: %d", len(result.Findings))
		for _, f := range result.Findings {
			t.Logf(" - Type: %s, Msg: %s", f.Type, f.Message)
		}
		t.Error("Expected giant_pr finding for PR #3")
	}
}
