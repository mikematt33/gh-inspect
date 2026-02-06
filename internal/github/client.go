package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/internal/cache"
)

// Ensure ClientWrapper satisfies the interface
var _ analysis.Client = (*ClientWrapper)(nil)

// ClientWrapper adapts the google/go-github client to the analysis.Client interface.
type ClientWrapper struct {
	client    *github.Client
	repoCache map[string]*github.Repository
	cacheMu   sync.RWMutex
	diskCache *cache.Cache
	useCache  bool
}

// ResolveToken attempts to find a GitHub token from:
// 1. Config file (if passed)
// 2. "gh auth token" command
// 3. GITHUB_TOKEN environment variable
func ResolveToken(configToken string) string {
	if configToken != "" {
		return configToken
	}

	// 2. Try gh CLI
	cmd := exec.Command("gh", "auth", "token")
	out, err := cmd.Output()
	if err == nil {
		token := strings.TrimSpace(string(out))
		if token != "" {
			return token
		}
	}

	// 2. Try Env var
	return os.Getenv("GITHUB_TOKEN")
}

// NewClient creates a new GitHub client wrapper.
func NewClient(token string) *ClientWrapper {
	return NewClientWithCache(token, true)
}

// NewClientWithCache creates a new GitHub client wrapper with cache control.
func NewClientWithCache(token string, useCache bool) *ClientWrapper {
	var ghClient *github.Client
	if token == "" {
		ghClient = github.NewClient(nil)
	} else {
		ghClient = github.NewClient(nil).WithAuthToken(token)
	}

	wrapper := &ClientWrapper{
		client:    ghClient,
		repoCache: make(map[string]*github.Repository),
		useCache:  useCache,
	}

	// Initialize disk cache if enabled
	if useCache {
		cachePath, err := cache.GetDefaultCachePath()
		if err == nil {
			c, err := cache.New(cachePath, time.Hour)
			if err == nil {
				wrapper.diskCache = c
			}
		}
	}

	return wrapper
}

// checkRateLimit inspects the response for rate limit headers
func (c *ClientWrapper) checkRateLimit(resp *github.Response) {
	if resp == nil {
		return
	}

	// Simple warning if low
	if resp.Rate.Remaining < 50 {
		fmt.Fprintf(os.Stderr, "⚠️ GitHub Rate Limit Low: %d/%d (Resets at %s)\n",
			resp.Rate.Remaining, resp.Rate.Limit, resp.Rate.Reset)
	}

	// If exhausted, we could sleep or error.
	// For this CLI, blocking is probably better than failing.
	if resp.Rate.Remaining == 0 {
		sleepDuration := time.Until(resp.Rate.Reset.Time)
		if sleepDuration > 0 {
			fmt.Fprintf(os.Stderr, "⛔ Rate limit exceeded. Sleeping for %v...\n", sleepDuration)
			time.Sleep(sleepDuration + 1*time.Second)
		}
	}
}

// GetRateLimit returns the current rate limit status
func (c *ClientWrapper) GetRateLimit(ctx context.Context) (*github.Rate, error) {
	rates, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, err
	}
	return startRate(rates.Core), nil
}

func startRate(r *github.Rate) *github.Rate {
	return r
}

// ListUserRepositories implements analysis.Client.
func (c *ClientWrapper) ListUserRepositories(ctx context.Context, user string, opts *github.RepositoryListOptions) ([]*github.Repository, error) {
	var allRepos []*github.Repository

	currentOpts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	if opts != nil {
		// Copy relevant fields from opts to currentOpts
		currentOpts.ListOptions = opts.ListOptions
		currentOpts.Visibility = opts.Visibility
		currentOpts.Affiliation = opts.Affiliation
		currentOpts.Type = opts.Type
		currentOpts.Sort = opts.Sort
		currentOpts.Direction = opts.Direction
	}

	for {
		repos, resp, err := c.client.Repositories.ListByAuthenticatedUser(ctx, currentOpts)
		if err != nil {
			return nil, err
		}
		c.checkRateLimit(resp)
		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}
		currentOpts.Page = resp.NextPage
	}
	return allRepos, nil
}

// GetUnderlyingClient returns the raw GitHub client for advanced operations
func (c *ClientWrapper) GetUnderlyingClient() *github.Client {
	return c.client
}

// GetPullRequests implements analysis.Client.
// Returns a single page of pull requests - callers should handle pagination if needed
func (c *ClientWrapper) GetPullRequests(ctx context.Context, owner, repo string, opts *github.PullRequestListOptions) ([]*github.PullRequest, error) {
	prs, resp, err := c.client.PullRequests.List(ctx, owner, repo, opts)
	if resp != nil {
		c.checkRateLimit(resp)
	}
	return prs, err
}

// GetReviews implements analysis.Client.
func (c *ClientWrapper) GetReviews(ctx context.Context, owner, repo string, number int, opts *github.ListOptions) ([]*github.PullRequestReview, error) {
	reviews, resp, err := c.client.PullRequests.ListReviews(ctx, owner, repo, number, opts)
	if resp != nil {
		c.checkRateLimit(resp)
	}
	return reviews, err
}

// ListCommitsSince implements Smart Pagination for commits
func (c *ClientWrapper) ListCommitsSince(ctx context.Context, owner, repo string, since time.Time) ([]*github.RepositoryCommit, error) {
	var allCommits []*github.RepositoryCommit
	opts := &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Since:       since, // GitHub API handles filtering naturally here
	}

	for {
		commits, resp, err := c.client.Repositories.ListCommits(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		allCommits = append(allCommits, commits...)

		if resp != nil {
			c.checkRateLimit(resp)
			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		} else {
			break
		}
	}
	return allCommits, nil
}

func (c *ClientWrapper) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	cacheKey := fmt.Sprintf("repo:%s/%s", owner, repo)

	// Check in-memory cache first
	c.cacheMu.RLock()
	if cached, ok := c.repoCache[cacheKey]; ok {
		c.cacheMu.RUnlock()
		return cached, nil
	}
	c.cacheMu.RUnlock()

	// Check disk cache if enabled
	if c.diskCache != nil {
		var cached github.Repository
		if found, err := c.diskCache.Get(cacheKey, &cached); err == nil && found {
			// Store in memory cache too
			c.cacheMu.Lock()
			c.repoCache[cacheKey] = &cached
			c.cacheMu.Unlock()
			return &cached, nil
		}
	}

	// Fetch from API
	r, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	// Store in memory cache
	c.cacheMu.Lock()
	c.repoCache[cacheKey] = r
	c.cacheMu.Unlock()

	// Store in disk cache if enabled
	if c.diskCache != nil {
		_ = c.diskCache.Set(cacheKey, r)
	}

	return r, nil
}

func (c *ClientWrapper) GetContent(ctx context.Context, owner, repo, path string) (*github.RepositoryContent, []*github.RepositoryContent, error) {
	fileContent, dirContent, _, err := c.client.Repositories.GetContents(ctx, owner, repo, path, nil)
	return fileContent, dirContent, err
}

func (c *ClientWrapper) GetCombinedStatus(ctx context.Context, owner, repo, ref string) (*github.CombinedStatus, error) {
	s, _, err := c.client.Repositories.GetCombinedStatus(ctx, owner, repo, ref, nil)
	return s, err
}

func (c *ClientWrapper) GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, error) {
	pr, resp, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if resp != nil {
		c.checkRateLimit(resp)
	}
	return pr, err
}

// GetIssues implements analysis.Client.
// Auto-paginates up to a reasonable limit to avoid excessive API calls
func (c *ClientWrapper) GetIssues(ctx context.Context, owner, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, error) {
	var allIssues []*github.Issue

	if opts.PerPage == 0 {
		opts.PerPage = 100
	}

	// Prevent unbounded pagination - caller should handle limits
	// This method will paginate automatically but not infinitely
	maxPages := 5 // Maximum 5 pages (500 issues with perPage=100)
	pageCount := 0

	for {
		issues, resp, err := c.client.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			c.checkRateLimit(resp)
		}

		for _, issue := range issues {
			if !issue.IsPullRequest() {
				allIssues = append(allIssues, issue)
			}
		}

		pageCount++
		if resp == nil || resp.NextPage == 0 || pageCount >= maxPages {
			break
		}
		opts.Page = resp.NextPage
	}
	return allIssues, nil
}

func (c *ClientWrapper) GetIssueComments(ctx context.Context, owner, repo string, number int, opts *github.IssueListCommentsOptions) ([]*github.IssueComment, error) {
	// Auto-paginate comments but with a reasonable limit
	var all []*github.IssueComment
	if opts == nil {
		opts = &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	} else if opts.PerPage == 0 {
		opts.PerPage = 100
	}

	maxPages := 3 // Limit to 300 comments per issue
	pageCount := 0

	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, err
		}
		c.checkRateLimit(resp)
		all = append(all, comments...)

		pageCount++
		if resp.NextPage == 0 || pageCount >= maxPages {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// GetWorkflowRuns implements analysis.Client.
func (c *ClientWrapper) GetWorkflowRuns(ctx context.Context, owner, repo string, opts *github.ListWorkflowRunsOptions) (*github.WorkflowRuns, *github.Response, error) {
	runs, resp, err := c.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if resp != nil {
		c.checkRateLimit(resp)
	}
	return runs, resp, err
}

// ListRepositories implements analysis.Client.
func (c *ClientWrapper) ListRepositories(ctx context.Context, org string, opts *github.RepositoryListByOrgOptions) ([]*github.Repository, error) {
	var allRepos []*github.Repository

	// Create a copy of options to modify page number without affecting caller's struct if meaningful,
	// but here we usually pass nil or generic options.
	// Actually, the interface requires us to handle pagination or just return one page?
	// The interface signature takes options.
	// Let's implement auto-pagination if no specific page is requested to be safe for "get all",
	// OR just pass through.
	//
	// For "Get Issues" we didn't auto-paginate in the client wrapper, we did it in the analyzer.
	// But getting ALL repos for an org is a primary operation.
	// Let's implement it as a simple pass-through for now to fit the pattern,
	// but usually we want all of them.

	repos, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
	if resp != nil {
		c.checkRateLimit(resp)
	}

	// If the caller wants all pages, they can't easily do it with this signature returning just []*Repo
	// unless we return the Response too (like we did for WorkflowRuns).
	// However, for Simplicity in this "Tier 5" step, I will implement auto-pagination inside this wrapper
	// if the caller passed nil options or default page=0 options, to return ALL repos.
	// This is slightly inconsistent but very convenient for the CLI logic.

	if err != nil {
		return nil, err
	}
	allRepos = append(allRepos, repos...)

	// If it's a multi-page response and we haven't manually restricted the page...
	if opts == nil || opts.Page == 0 {
		for resp.NextPage != 0 {
			nextOpts := &github.RepositoryListByOrgOptions{
				ListOptions: github.ListOptions{
					Page:    resp.NextPage,
					PerPage: 100,
				},
			}
			// Preserve other filters if opts was provided
			if opts != nil {
				nextOpts.Type = opts.Type
			}

			repos, nextResp, err := c.client.Repositories.ListByOrg(ctx, org, nextOpts)
			if err != nil {
				return nil, err
			}
			c.checkRateLimit(nextResp)
			allRepos = append(allRepos, repos...)
			resp = nextResp
		}
	}

	return allRepos, nil
}

// GetTree gets a git tree (efficient for checking multiple files)
func (c *ClientWrapper) GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*github.Tree, error) {
	tree, _, err := c.client.Git.GetTree(ctx, owner, repo, sha, recursive)
	return tree, err
}

// Note: Future optimization opportunity - Implement GraphQL queries for batching
// GraphQL could combine multiple REST calls into single queries, e.g.:
// - Fetch repo metadata + branch protection + CI status in one query
// - Batch PR queries with review data included
// - Get multiple file contents or tree in one query
// This would significantly reduce API calls for analyzers that need related data
