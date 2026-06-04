package github

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

// Credential pairs a TokenSource with independently tracked rate-limit state.
type Credential struct {
	source TokenSource

	mu        sync.Mutex
	remaining int
	limit     int
	reset     time.Time
	primed    bool // true once we have observed at least one rate-limit reading
	exhausted bool
	invalid   bool
}

func newCredential(src TokenSource) *Credential {
	return &Credential{source: src, remaining: 5000, limit: 5000}
}

// Name returns the credential's friendly identifier.
func (c *Credential) Name() string { return c.source.Name() }

// Kind returns the credential type ("pat" or "app").
func (c *Credential) Kind() string { return c.source.Kind() }

// client builds a go-github client authenticated with this credential's current
// token. App credentials refresh their installation token transparently.
func (c *Credential) client(ctx context.Context) (*github.Client, error) {
	token, err := c.source.Token(ctx)
	if err != nil {
		c.markInvalid()
		return nil, err
	}
	return github.NewClient(nil).WithAuthToken(token), nil
}

func (c *Credential) snapshot() models.TokenStatusSummary {
	c.mu.Lock()
	defer c.mu.Unlock()
	return models.TokenStatusSummary{
		Name:      c.source.Name(),
		Kind:      c.source.Kind(),
		Remaining: c.remaining,
		Limit:     c.limit,
		Exhausted: c.exhausted,
		Invalid:   c.invalid,
	}
}

func (c *Credential) markInvalid() {
	c.mu.Lock()
	c.invalid = true
	c.mu.Unlock()
}

// observe updates rate-limit tracking from a GitHub API response.
func (c *Credential) observe(resp *github.Response) {
	if resp == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.primed = true
	c.remaining = resp.Rate.Remaining
	c.limit = resp.Rate.Limit
	c.reset = resp.Rate.Reset.Time
	if c.remaining <= 0 {
		c.exhausted = true
	}
}

// markExhausted flags the credential as out of quota until its reset time.
func (c *Credential) markExhausted(reset time.Time) {
	c.mu.Lock()
	c.exhausted = true
	c.remaining = 0
	if !reset.IsZero() {
		c.reset = reset
	}
	c.mu.Unlock()
}

// available reports the usable remaining quota, accounting for reset windows.
func (c *Credential) available() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.invalid {
		return -1
	}
	// If the reset window has passed, assume quota has replenished.
	if c.exhausted && !c.reset.IsZero() && time.Now().After(c.reset) {
		c.exhausted = false
		c.remaining = c.limit
	}
	if c.exhausted {
		return 0
	}
	return c.remaining
}

// Pool manages a set of credentials and selects the best one for each request,
// rotating away from exhausted or rate-limited credentials.
type Pool struct {
	creds []*Credential
}

// NewPool builds a credential pool from token sources.
func NewPool(sources ...TokenSource) *Pool {
	p := &Pool{}
	for _, s := range sources {
		if s != nil {
			p.creds = append(p.creds, newCredential(s))
		}
	}
	return p
}

// Len returns the number of credentials in the pool.
func (p *Pool) Len() int { return len(p.creds) }

// Best returns the credential with the most remaining quota that is neither
// exhausted nor invalid. It returns nil if no usable credential exists.
func (p *Pool) Best() *Credential {
	var best *Credential
	bestQuota := -1
	for _, c := range p.creds {
		avail := c.available()
		if avail <= 0 {
			continue
		}
		if avail > bestQuota {
			bestQuota = avail
			best = c
		}
	}
	return best
}

// EarliestReset returns the soonest reset time among exhausted credentials,
// used to decide how long to wait when all credentials are depleted.
func (p *Pool) EarliestReset() (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, c := range p.creds {
		c.mu.Lock()
		if c.exhausted && !c.invalid && !c.reset.IsZero() {
			if !found || c.reset.Before(earliest) {
				earliest = c.reset
				found = true
			}
		}
		c.mu.Unlock()
	}
	return earliest, found
}

// Summaries returns a status snapshot for every credential, sorted by remaining
// quota descending for stable reporting.
func (p *Pool) Summaries() []models.TokenStatusSummary {
	out := make([]models.TokenStatusSummary, 0, len(p.creds))
	for _, c := range p.creds {
		out = append(out, c.snapshot())
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Remaining > out[j].Remaining
	})
	return out
}

// TotalAvailable sums the usable remaining quota across all credentials.
func (p *Pool) TotalAvailable() int {
	total := 0
	for _, c := range p.creds {
		if a := c.available(); a > 0 {
			total += a
		}
	}
	return total
}

// Prime queries the live rate limit for each credential so that quota tracking
// reflects reality before a scan begins. Errors mark a credential invalid but
// do not abort priming of the others.
func (p *Pool) Prime(ctx context.Context) {
	for _, c := range p.creds {
		client, err := c.client(ctx)
		if err != nil {
			continue
		}
		rl, resp, err := client.RateLimit.Get(ctx)
		if err != nil {
			c.markInvalid()
			continue
		}
		if resp != nil {
			c.observe(resp)
		}
		if rl != nil && rl.Core != nil {
			c.mu.Lock()
			c.remaining = rl.Core.Remaining
			c.limit = rl.Core.Limit
			c.reset = rl.Core.Reset.Time
			c.primed = true
			c.exhausted = rl.Core.Remaining <= 0
			c.mu.Unlock()
		}
	}
}
