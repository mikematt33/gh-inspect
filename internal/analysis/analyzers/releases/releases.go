package releases

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Name() string {
	return "releases"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	var metrics []models.Metric
	var findings []models.Finding

	// Fetch releases in the time window
	opts := &github.ListOptions{PerPage: 100}
	allReleases, _, err := client.GetUnderlyingClient().Repositories.ListReleases(ctx, repo.Owner, repo.Name, opts)
	if err != nil {
		return models.AnalyzerResult{Name: a.Name()}, err
	}

	// Filter releases since cfg.Since using PublishedAt (or CreatedAt as fallback)
	var recentReleases []*github.RepositoryRelease
	for _, release := range allReleases {
		// Use PublishedAt if available, otherwise fall back to CreatedAt
		releaseTime := release.GetPublishedAt()
		if releaseTime.IsZero() {
			releaseTime = release.GetCreatedAt()
		}
		if releaseTime.After(cfg.Since) {
			recentReleases = append(recentReleases, release)
		}
	}

	if len(recentReleases) == 0 {
		metrics = append(metrics, models.Metric{
			Key:          "releases_in_window",
			Value:        0,
			DisplayValue: "0",
			Description:  "No releases in time window",
		})

		if len(allReleases) > 0 {
			// There are releases, just not recent ones
			lastRelease := allReleases[0]
			daysSince := time.Since(lastRelease.CreatedAt.Time).Hours() / 24
			metrics = append(metrics, models.Metric{
				Key:          "days_since_last_release",
				Value:        daysSince,
				DisplayValue: fmt.Sprintf("%.0f days", daysSince),
				Description:  "Days since last release",
			})

			if daysSince > 180 {
				findings = append(findings, models.Finding{
					Type:        "stale_releases",
					Severity:    models.SeverityLow,
					Message:     fmt.Sprintf("No release in %.0f days", daysSince),
					Actionable:  true,
					Remediation: "Consider creating a new release.",
				})
			}
		} else {
			findings = append(findings, models.Finding{
				Type:        "no_releases",
				Severity:    models.SeverityInfo,
				Message:     "Repository has no releases",
				Actionable:  true,
				Remediation: "Consider using GitHub releases for version tracking.",
			})
		}

		return models.AnalyzerResult{
			Name:     a.Name(),
			Metrics:  metrics,
			Findings: findings,
		}, nil
	}

	// Calculate release frequency
	days := time.Since(cfg.Since).Hours() / 24
	releaseFrequency := float64(len(recentReleases)) / days * 30 // per month

	metrics = append(metrics, models.Metric{
		Key:          "releases_in_window",
		Value:        float64(len(recentReleases)),
		DisplayValue: fmt.Sprintf("%d", len(recentReleases)),
		Description:  "Releases in time window",
	})
	metrics = append(metrics, models.Metric{
		Key:          "release_frequency_monthly",
		Value:        releaseFrequency,
		Unit:         "releases/month",
		DisplayValue: fmt.Sprintf("%.1f/month", releaseFrequency),
		Description:  "Average releases per month",
	})

	// Time between releases
	if len(recentReleases) > 1 {
		var totalTimeBetween time.Duration
		for i := 0; i < len(recentReleases)-1; i++ {
			timeBetween := recentReleases[i].CreatedAt.Sub(recentReleases[i+1].CreatedAt.Time)
			totalTimeBetween += timeBetween
		}
		avgDaysBetween := totalTimeBetween.Hours() / 24 / float64(len(recentReleases)-1)
		metrics = append(metrics, models.Metric{
			Key:          "avg_days_between_releases",
			Value:        avgDaysBetween,
			Unit:         "days",
			DisplayValue: fmt.Sprintf("%.0f days", avgDaysBetween),
			Description:  "Average days between releases",
		})
	}

	// Pre-release vs stable ratio
	preReleaseCount := 0
	hasChangelogCount := 0
	semverCompliant := 0
	semverPattern := regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

	for _, release := range recentReleases {
		if release.GetPrerelease() {
			preReleaseCount++
		}
		if len(release.GetBody()) > 50 {
			hasChangelogCount++
		}
		if semverPattern.MatchString(release.GetTagName()) {
			semverCompliant++
		}
	}

	preReleaseRatio := float64(preReleaseCount) / float64(len(recentReleases)) * 100
	changelogRatio := float64(hasChangelogCount) / float64(len(recentReleases)) * 100
	semverRatio := float64(semverCompliant) / float64(len(recentReleases)) * 100

	metrics = append(metrics, models.Metric{
		Key:          "prerelease_ratio",
		Value:        preReleaseRatio,
		Unit:         "percent",
		DisplayValue: fmt.Sprintf("%.0f%%", preReleaseRatio),
		Description:  "Percentage of pre-releases",
	})
	metrics = append(metrics, models.Metric{
		Key:          "changelog_coverage",
		Value:        changelogRatio,
		Unit:         "percent",
		DisplayValue: fmt.Sprintf("%.0f%%", changelogRatio),
		Description:  "Releases with release notes",
	})
	metrics = append(metrics, models.Metric{
		Key:          "semver_compliance",
		Value:        semverRatio,
		Unit:         "percent",
		DisplayValue: fmt.Sprintf("%.0f%%", semverRatio),
		Description:  "Semantic versioning compliance",
	})

	// Deployment velocity metrics
	if len(allReleases) > 0 {
		// Time since last deployment
		lastRelease := allReleases[0]
		daysSinceRelease := time.Since(lastRelease.CreatedAt.Time).Hours() / 24
		metrics = append(metrics, models.Metric{
			Key:          "days_since_last_release",
			Value:        daysSinceRelease,
			Unit:         "days",
			DisplayValue: fmt.Sprintf("%.0f days", daysSinceRelease),
			Description:  "Days since last release",
		})

		// Deployment consistency (calculate standard deviation of release intervals)
		if len(recentReleases) >= 3 {
			var intervals []float64
			for i := 0; i < len(recentReleases)-1 && i < 10; i++ {
				interval := recentReleases[i].CreatedAt.Sub(recentReleases[i+1].CreatedAt.Time).Hours() / 24
				intervals = append(intervals, interval)
			}

			// Calculate mean
			var sum float64
			for _, interval := range intervals {
				sum += interval
			}
			mean := sum / float64(len(intervals))

			// Calculate standard deviation
			var varianceSum float64
			for _, interval := range intervals {
				varianceSum += (interval - mean) * (interval - mean)
			}
			stdDev := 0.0
			if len(intervals) > 0 {
				variance := varianceSum / float64(len(intervals))
				stdDev = math.Sqrt(variance)
			}

			// Coefficient of variation (CV) - lower is more consistent
			cv := 0.0
			if mean > 0 {
				cv = (stdDev / mean) * 100
			}

			metrics = append(metrics, models.Metric{
				Key:          "release_consistency",
				Value:        cv,
				Unit:         "cv%",
				DisplayValue: fmt.Sprintf("%.0f%%", cv),
				Description:  "Release consistency (lower = more consistent)",
			})
		}

		// Check for potential rollbacks (releases published close together)
		potentialRollbacks := 0
		for i := 0; i < len(recentReleases)-1; i++ {
			timeBetween := recentReleases[i].CreatedAt.Sub(recentReleases[i+1].CreatedAt.Time).Hours()
			// If released within 2 hours, might be a hotfix/rollback
			if timeBetween < 2 {
				potentialRollbacks++
			}
		}

		if potentialRollbacks > 0 {
			metrics = append(metrics, models.Metric{
				Key:          "rapid_releases",
				Value:        float64(potentialRollbacks),
				Unit:         "count",
				DisplayValue: fmt.Sprintf("%d", potentialRollbacks),
				Description:  "Releases within 2h of previous (potential hotfixes)",
			})
		}
	}

	// Production readiness indicators
	stableReleases := len(recentReleases) - preReleaseCount
	if stableReleases > 0 {
		metrics = append(metrics, models.Metric{
			Key:          "stable_releases",
			Value:        float64(stableReleases),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", stableReleases),
			Description:  "Stable (non-prerelease) releases",
		})
	}

	// Findings
	if changelogRatio < 50 {
		findings = append(findings, models.Finding{
			Type:        "missing_changelogs",
			Severity:    models.SeverityLow,
			Message:     "Many releases lack detailed release notes",
			Actionable:  true,
			Remediation: "Add changelog or release notes to releases.",
		})
	}

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
