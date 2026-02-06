package analysis

import (
	"context"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

// Config defines the scope of analysis
type Config struct {
	Since       time.Time         // Lookback window (e.g., 30 days)
	IncludeDeep bool              // If true, perform costlier scans
	DepthConfig DepthConfig       // Depth configuration with limits
	OutputMode  models.OutputMode // How to present findings (suggestive, observational, statistical)
}

// Analyzer is the core interface that all inspection logic must implement.
type Analyzer interface {
	Name() string
	// Analyze executes the inspection logic against a specific repository.
	Analyze(ctx context.Context, client Client, repo TargetRepository, cfg Config) (models.AnalyzerResult, error)
}

// TargetRepository contains the minimal context needed to locate a repo.
type TargetRepository struct {
	Owner string
	Name  string
}

// Client defines the subset of GitHub API methods needed by Analyzers.
type Client interface {
	GetPullRequests(ctx context.Context, owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error)
	GetReviews(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.PullRequestReview, error)
	ListCommitsSince(ctx context.Context, owner, repo string, since time.Time) ([]*github.RepositoryCommit, error)
	GetRateLimit(ctx context.Context) (*github.Rate, error)

	// Tier 2 additions
	GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error)
	GetContent(ctx context.Context, owner, repo, path string) (*github.RepositoryContent, []*github.RepositoryContent, error)
	GetCombinedStatus(ctx context.Context, owner, repo, ref string) (*github.CombinedStatus, error)

	// Tier 3 additions
	GetIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, error)
	GetIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, error)

	// Tier 4 additions
	GetWorkflowRuns(ctx context.Context, owner, repo string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error)

	// Tier 5 additions (Org Level)
	ListRepositories(ctx context.Context, org string, opts *github.RepositoryListByOrgOptions) ([]*github.Repository, error)
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error)

	// GetUnderlyingClient exposes the raw GitHub client for advanced operations not yet abstracted
	GetUnderlyingClient() *github.Client

	// GetTree gets a git tree for efficient multi-file checking
	GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*github.Tree, error)
}
