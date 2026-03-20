package issuehygiene

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
)

type mockIssueClient struct {
	issuesByState map[string][][]*github.Issue
	issueCalls    map[string]int
}

func (m *mockIssueClient) GetPullRequests(ctx context.Context, owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	return nil, nil
}

func (m *mockIssueClient) GetReviews(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.PullRequestReview, error) {
	return nil, nil
}

func (m *mockIssueClient) ListCommitsSince(ctx context.Context, owner, repo string, since time.Time) ([]*github.RepositoryCommit, error) {
	return nil, nil
}

func (m *mockIssueClient) GetRateLimit(ctx context.Context) (*github.Rate, error) {
	return &github.Rate{}, nil
}

func (m *mockIssueClient) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	return nil, nil
}

func (m *mockIssueClient) GetContent(ctx context.Context, owner, repo, path string) (*github.RepositoryContent, []*github.RepositoryContent, error) {
	return nil, nil, nil
}

func (m *mockIssueClient) GetCombinedStatus(ctx context.Context, owner, repo, ref string) (*github.CombinedStatus, error) {
	return nil, nil
}

func (m *mockIssueClient) GetIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	state := opts.State
	if m.issueCalls == nil {
		m.issueCalls = make(map[string]int)
	}
	page := opts.Page
	if page == 0 {
		page = 1
	}
	m.issueCalls[state]++

	pages := m.issuesByState[state]
	if page < 1 || page > len(pages) {
		return nil, &github.Response{}, nil
	}

	nextPage := 0
	if page < len(pages) {
		nextPage = page + 1
	}

	return pages[page-1], &github.Response{NextPage: nextPage}, nil
}

func (m *mockIssueClient) GetIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, error) {
	return nil, nil
}

func (m *mockIssueClient) GetWorkflowRuns(ctx context.Context, owner, repo string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error) {
	return nil, nil, nil
}

func (m *mockIssueClient) ListRepositories(ctx context.Context, org string, opts *github.RepositoryListByOrgOptions) ([]*github.Repository, error) {
	return nil, nil
}

func (m *mockIssueClient) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	return nil, nil
}

func (m *mockIssueClient) GetUnderlyingClient() *github.Client {
	return nil
}

func (m *mockIssueClient) GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*github.Tree, error) {
	return nil, nil
}

func TestAnalyzerRespectsMaxIssuesAcrossPages(t *testing.T) {
	now := time.Now()
	issue := func(number int) *github.Issue {
		created := github.Timestamp{Time: now.Add(-48 * time.Hour)}
		updated := github.Timestamp{Time: now.Add(-24 * time.Hour)}
		return &github.Issue{
			Number:    github.Int(number),
			CreatedAt: &created,
			UpdatedAt: &updated,
		}
	}

	client := &mockIssueClient{
		issuesByState: map[string][][]*github.Issue{
			"open": {
				{issue(1), issue(2)},
				{issue(3), issue(4)},
			},
			"closed": {
				{issue(10), issue(11)},
				{issue(12), issue(13)},
			},
		},
	}

	analyzer := New(30, 90)
	result, err := analyzer.Analyze(context.Background(), client, analysis.TargetRepository{Owner: "o", Name: "r"}, analysis.Config{
		Since: time.Now().Add(-7 * 24 * time.Hour),
		DepthConfig: analysis.DepthConfig{
			MaxIssues: 3,
		},
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	metrics := make(map[string]float64, len(result.Metrics))
	for _, metric := range result.Metrics {
		metrics[metric.Key] = metric.Value
	}

	if metrics["open_issues_total"] != 3 {
		t.Fatalf("expected 3 open issues, got %v", metrics["open_issues_total"])
	}
	if metrics["closed_issues_in_window"] != 3 {
		t.Fatalf("expected 3 closed issues, got %v", metrics["closed_issues_in_window"])
	}
	if client.issueCalls["open"] != 2 {
		t.Fatalf("expected 2 open issue page calls, got %d", client.issueCalls["open"])
	}
	if client.issueCalls["closed"] != 2 {
		t.Fatalf("expected 2 closed issue page calls, got %d", client.issueCalls["closed"])
	}
}
