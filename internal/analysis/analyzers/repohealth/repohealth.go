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

	// 2. Check Key Files
	keyFiles := []struct {
		Path     string
		Severity models.Severity
		ScoreDed int
		Found    bool
	}{
		{"LICENSE", models.SeverityHigh, 30, false},
		{"README.md", models.SeverityMedium, 10, false},
		{"CONTRIBUTING.md", models.SeverityLow, 5, false},
		{"SECURITY.md", models.SeverityMedium, 15, false},
		{".github/CODEOWNERS", models.SeverityLow, 5, false},
	}

	for i := range keyFiles {
		f := &keyFiles[i]
		// Try root
		_, _, err := client.GetContent(ctx, repo.Owner, repo.Name, f.Path)
		if err == nil {
			f.Found = true
			continue
		}

		// Common alternative locations (e.g. docs/ or .github/ for SECURITY.md)
		if f.Path == "SECURITY.md" {
			_, _, err := client.GetContent(ctx, repo.Owner, repo.Name, ".github/SECURITY.md")
			if err == nil {
				f.Found = true
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

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
