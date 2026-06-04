package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v60/github"
)

// ActionsClient provides GitHub Actions API access backed by a credential Pool.
// Each request is routed to the credential with the most remaining quota, and
// rate-limited responses (403/429) trigger rotation to another credential or a
// bounded wait until the soonest reset.
type ActionsClient struct {
	pool *Pool
}

// NewActionsClient wraps a credential pool for Actions analytics.
func NewActionsClient(pool *Pool) *ActionsClient {
	return &ActionsClient{pool: pool}
}

// Pool exposes the underlying credential pool (for status reporting).
func (c *ActionsClient) Pool() *Pool { return c.pool }

// apiCall is a single GitHub API operation parameterized by the client to use.
type apiCall func(gh *github.Client) (*github.Response, error)

// do selects the best credential, executes the call, and retries on rate-limit
// errors by rotating credentials or waiting for a reset.
func (c *ActionsClient) do(ctx context.Context, call apiCall) error {
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		cred := c.pool.Best()
		if cred == nil {
			// All credentials exhausted: wait for the soonest reset, if known.
			reset, ok := c.pool.EarliestReset()
			if !ok {
				return fmt.Errorf("no usable GitHub credentials remain (all exhausted or invalid)")
			}
			wait := time.Until(reset)
			if wait <= 0 {
				continue
			}
			fmt.Fprintf(os.Stderr, "⛔ All tokens rate-limited. Waiting %v for reset...\n", wait.Round(time.Second))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait + time.Second):
			}
			continue
		}

		gh, err := cred.client(ctx)
		if err != nil {
			continue // credential marked invalid; try the next best
		}

		resp, err := call(gh)
		if resp != nil {
			cred.observe(resp)
		}
		if err == nil {
			return nil
		}

		if isRateLimitErr(err, resp) {
			reset := rateLimitReset(err, resp)
			cred.markExhausted(reset)
			continue // rotate to another credential
		}
		return err
	}
	return fmt.Errorf("exceeded retry budget due to repeated rate limiting")
}

func isRateLimitErr(err error, resp *github.Response) bool {
	var rle *github.RateLimitError
	if errors.As(err, &rle) {
		return true
	}
	var abuse *github.AbuseRateLimitError
	if errors.As(err, &abuse) {
		return true
	}
	if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) {
		return resp.Rate.Remaining == 0
	}
	return false
}

func rateLimitReset(err error, resp *github.Response) time.Time {
	var rle *github.RateLimitError
	if errors.As(err, &rle) {
		return rle.Rate.Reset.Time
	}
	if resp != nil {
		return resp.Rate.Reset.Time
	}
	return time.Now().Add(time.Minute)
}

// ListWorkflows returns all workflows defined in a repository.
func (c *ActionsClient) ListWorkflows(ctx context.Context, owner, repo string) ([]*github.Workflow, error) {
	var all []*github.Workflow
	opts := &github.ListOptions{PerPage: 100}
	for {
		var page *github.Workflows
		var nextPage int
		err := c.do(ctx, func(gh *github.Client) (*github.Response, error) {
			w, resp, err := gh.Actions.ListWorkflows(ctx, owner, repo, opts)
			if err != nil {
				return resp, err
			}
			page = w
			if resp != nil {
				nextPage = resp.NextPage
			}
			return resp, nil
		})
		if err != nil {
			return nil, err
		}
		if page != nil {
			all = append(all, page.Workflows...)
		}
		if nextPage == 0 {
			break
		}
		opts.Page = nextPage
	}
	return all, nil
}

// ListWorkflowRuns returns workflow runs created within the given window,
// capped at maxRuns (0 means no cap). The optional created filter follows the
// GitHub search-date syntax (e.g. ">=2024-01-01").
func (c *ActionsClient) ListWorkflowRuns(ctx context.Context, owner, repo, created string, maxRuns int) ([]*github.WorkflowRun, int, error) {
	var all []*github.WorkflowRun
	total := 0
	opts := &github.ListWorkflowRunsOptions{
		Created:     created,
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		var page *github.WorkflowRuns
		var nextPage int
		err := c.do(ctx, func(gh *github.Client) (*github.Response, error) {
			r, resp, err := gh.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
			if err != nil {
				return resp, err
			}
			page = r
			if resp != nil {
				nextPage = resp.NextPage
			}
			return resp, nil
		})
		if err != nil {
			return nil, 0, err
		}
		if page != nil {
			if page.TotalCount != nil && total == 0 {
				total = *page.TotalCount
			}
			all = append(all, page.WorkflowRuns...)
		}
		if nextPage == 0 || (maxRuns > 0 && len(all) >= maxRuns) {
			break
		}
		opts.Page = nextPage
	}
	if maxRuns > 0 && len(all) > maxRuns {
		all = all[:maxRuns]
	}
	return all, total, nil
}

// CountWorkflowRuns returns just the total run count for a repository using a
// single cheap request (PerPage=1).
func (c *ActionsClient) CountWorkflowRuns(ctx context.Context, owner, repo, created string) (int, error) {
	opts := &github.ListWorkflowRunsOptions{
		Created:     created,
		ListOptions: github.ListOptions{PerPage: 1},
	}
	total := 0
	err := c.do(ctx, func(gh *github.Client) (*github.Response, error) {
		r, resp, err := gh.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
		if err != nil {
			return resp, err
		}
		if r != nil && r.TotalCount != nil {
			total = *r.TotalCount
		}
		return resp, nil
	})
	return total, err
}

// ListWorkflowJobs returns all jobs for a workflow run.
func (c *ActionsClient) ListWorkflowJobs(ctx context.Context, owner, repo string, runID int64) ([]*github.WorkflowJob, error) {
	var all []*github.WorkflowJob
	opts := &github.ListWorkflowJobsOptions{
		Filter:      "latest",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		var page *github.Jobs
		var nextPage int
		err := c.do(ctx, func(gh *github.Client) (*github.Response, error) {
			j, resp, err := gh.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
			if err != nil {
				return resp, err
			}
			page = j
			if resp != nil {
				nextPage = resp.NextPage
			}
			return resp, nil
		})
		if err != nil {
			return nil, err
		}
		if page != nil {
			all = append(all, page.Jobs...)
		}
		if nextPage == 0 {
			break
		}
		opts.Page = nextPage
	}
	return all, nil
}

// ListOrgRepositories returns all non-archived repositories for an organization.
func (c *ActionsClient) ListOrgRepositories(ctx context.Context, org string) ([]*github.Repository, error) {
	var all []*github.Repository
	opts := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		var page []*github.Repository
		var nextPage int
		err := c.do(ctx, func(gh *github.Client) (*github.Response, error) {
			repos, resp, err := gh.Repositories.ListByOrg(ctx, org, opts)
			if err != nil {
				return resp, err
			}
			page = repos
			if resp != nil {
				nextPage = resp.NextPage
			}
			return resp, nil
		})
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if nextPage == 0 {
			break
		}
		opts.Page = nextPage
	}
	return all, nil
}

// GetFileContent fetches a single file's decoded content (used to read workflow
// YAML for trigger and cron parsing). Returns an empty string if absent.
func (c *ActionsClient) GetFileContent(ctx context.Context, owner, repo, path string) (string, error) {
	var content string
	err := c.do(ctx, func(gh *github.Client) (*github.Response, error) {
		fc, _, resp, err := gh.Repositories.GetContents(ctx, owner, repo, path, nil)
		if err != nil {
			return resp, err
		}
		if fc != nil {
			s, derr := fc.GetContent()
			if derr != nil {
				return resp, derr
			}
			content = s
		}
		return resp, nil
	})
	if err != nil {
		// Missing file is not fatal for analytics.
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
			return "", nil
		}
		return "", err
	}
	return content, nil
}
