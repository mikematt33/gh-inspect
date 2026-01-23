package repohealth

import (
	"context"
	"fmt"

	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Name() string {
	return "repo-health"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	// 1. Get fundamental repo info (for default branch name)
	r, err := client.GetRepository(ctx, repo.Owner, repo.Name)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}
	defaultBranch := r.GetDefaultBranch()
	if defaultBranch == "" {
		defaultBranch = "main" // fallback
	}

	var findings []models.Finding
	var metrics []models.Metric
	healthScore := 100

	// 2. Check Key Files efficiently using git tree API (1 API call instead of 6+)
	keyFiles := []struct {
		Path     string
		AltPaths []string // Alternative locations
		Severity models.Severity
		ScoreDed int
		Found    bool
	}{
		{"LICENSE", nil, models.SeverityHigh, 30, false},
		{"README.md", nil, models.SeverityMedium, 10, false},
		{"CONTRIBUTING.md", nil, models.SeverityLow, 5, false},
		{"SECURITY.md", []string{".github/SECURITY.md"}, models.SeverityMedium, 15, false},
		{"CODE_OF_CONDUCT.md", []string{".github/CODE_OF_CONDUCT.md"}, models.SeverityLow, 5, false},
		{".github/CODEOWNERS", nil, models.SeverityLow, 5, false},
	}

	// Use git tree API to check all files at once (much more efficient)
	tree, err := client.GetTree(ctx, repo.Owner, repo.Name, defaultBranch, true)
	if err == nil && tree != nil {
		// Build a set of all paths in the tree
		pathSet := make(map[string]bool)
		for _, entry := range tree.Entries {
			if entry.Path != nil {
				pathSet[*entry.Path] = true
			}
		}

		// Check which key files exist
		for i := range keyFiles {
			f := &keyFiles[i]
			// Check primary path
			if pathSet[f.Path] {
				f.Found = true
				continue
			}
			// Check alternative paths
			for _, altPath := range f.AltPaths {
				if pathSet[altPath] {
					f.Found = true
					break
				}
			}
		}
	} else {
		// Fallback to individual checks if tree API fails (e.g., empty repo)
		for i := range keyFiles {
			f := &keyFiles[i]
			// Try root
			_, _, err := client.GetContent(ctx, repo.Owner, repo.Name, f.Path)
			if err == nil {
				f.Found = true
				continue
			}

			// Try alternative paths
			for _, altPath := range f.AltPaths {
				_, _, err := client.GetContent(ctx, repo.Owner, repo.Name, altPath)
				if err == nil {
					f.Found = true
					break
				}
			}
		}
	}

	for _, f := range keyFiles {
		if !f.Found {
			healthScore -= f.ScoreDed
			findings = append(findings, models.Finding{
				Type:        "missing_file",
				Severity:    f.Severity,
				Message:     fmt.Sprintf("Missing key file: %s", f.Path),
				Actionable:  true,
				Remediation: fmt.Sprintf("Add a %s file to the repository root.", f.Path),
			})
		}
	}

	// 3. Check CI Status on Default Branch
	combinedStatus, err := client.GetCombinedStatus(ctx, repo.Owner, repo.Name, defaultBranch)
	if err == nil {
		// State: pending, success, failure, error
		state := combinedStatus.GetState()
		metrics = append(metrics, models.Metric{
			Key:          "ci_status",
			Value:        0, // value not numeric really
			Unit:         "state",
			DisplayValue: state,
			Description:  fmt.Sprintf("CI Status for %s", defaultBranch),
		})

		if state == "failure" || state == "error" {
			healthScore -= 20
			findings = append(findings, models.Finding{
				Type:        "ci_failure",
				Severity:    models.SeverityHigh,
				Message:     fmt.Sprintf("CI is failing on default branch (%s)", defaultBranch),
				Actionable:  true,
				Remediation: "Fix the build break immediately.",
			})
		} else if state == "pending" {
			// Could be stuck or just running.
		} else if combinedStatus.GetTotalCount() == 0 {
			// No statuses at all?
			healthScore -= 10
			findings = append(findings, models.Finding{
				Type:        "ci_missing",
				Severity:    models.SeverityMedium,
				Message:     "No CI statuses found on default branch",
				Actionable:  true,
				Remediation: "Configure GitHub Actions or an external CI provider.",
			})
		}
	}

	// Normalize score
	if healthScore < 0 {
		healthScore = 0
	}

	metrics = append(metrics, models.Metric{
		Key:          "health_score",
		Value:        float64(healthScore),
		Unit:         "points",
		DisplayValue: fmt.Sprintf("%d/100", healthScore),
		Description:  "Calculated repo health score based on files and CI",
	})

	// 4. Check Branch Protection
	protection, _, protErr := client.GetUnderlyingClient().Repositories.GetBranchProtection(ctx, repo.Owner, repo.Name, defaultBranch)
	if protErr == nil && protection != nil {
		metrics = append(metrics, models.Metric{
			Key:          "branch_protection_enabled",
			Value:        1,
			DisplayValue: "Yes",
			Description:  "Branch protection rules configured",
		})
		if protection.RequiredPullRequestReviews != nil {
			metrics = append(metrics, models.Metric{
				Key:          "requires_pr_reviews",
				Value:        1,
				DisplayValue: "Yes",
				Description:  "Requires PR reviews before merge",
			})
		}
		if protection.RequiredStatusChecks != nil {
			metrics = append(metrics, models.Metric{
				Key:          "requires_status_checks",
				Value:        1,
				DisplayValue: "Yes",
				Description:  "Requires status checks to pass",
			})
		}
	} else {
		metrics = append(metrics, models.Metric{
			Key:          "branch_protection_enabled",
			Value:        0,
			DisplayValue: "No",
			Description:  "No branch protection configured",
		})
		// healthScore -= 15 // This assignment has no effect
		findings = append(findings, models.Finding{
			Type:        "no_branch_protection",
			Severity:    models.SeverityMedium,
			Message:     fmt.Sprintf("No branch protection on %s", defaultBranch),
			Actionable:  true,
			Remediation: "Enable branch protection rules.",
		})
	}

	// 5. Check dependency files (reuse tree from earlier if available)
	depFiles := []string{"package.json", "requirements.txt", "pom.xml", "build.gradle", "go.mod", "Cargo.toml", "Gemfile"}
	depFound := false
	if tree != nil {
		// Reuse the tree we already fetched
		for _, df := range depFiles {
			for _, entry := range tree.Entries {
				if entry.Path != nil && *entry.Path == df {
					depFound = true
					break
				}
			}
			if depFound {
				break
			}
		}
	} else {
		// Fallback to individual checks
		for _, df := range depFiles {
			if _, _, err := client.GetContent(ctx, repo.Owner, repo.Name, df); err == nil {
				depFound = true
				break
			}
		}
	}
	metrics = append(metrics, models.Metric{
		Key:          "has_dependency_management",
		Value:        map[bool]float64{true: 1, false: 0}[depFound],
		DisplayValue: map[bool]string{true: "Yes", false: "No"}[depFound],
		Description:  "Uses dependency management",
	})

	// Add default branch metric
	metrics = append(metrics, models.Metric{
		Key:          "default_branch",
		DisplayValue: defaultBranch,
		Description:  "Default branch name",
	})

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
