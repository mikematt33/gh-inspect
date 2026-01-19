package security

import (
	"context"
	"fmt"

	"github.com/google/go-github/v60/github"
	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Name() string {
	return "security"
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	var metrics []models.Metric
	var findings []models.Finding

	// Note: These APIs require GitHub Advanced Security or public repos
	// Gracefully handle permission errors

	// 1. Dependabot Alerts
	state := "open"
	dependabotAlerts, _, err := client.GetUnderlyingClient().Dependabot.ListRepoAlerts(ctx, repo.Owner, repo.Name, &github.ListAlertsOptions{
		State: &state,
	})

	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	if err == nil {
		for _, alert := range dependabotAlerts {
			severity := alert.SecurityAdvisory.GetSeverity()
			switch severity {
			case "critical":
				criticalCount++
			case "high":
				highCount++
			case "medium":
				mediumCount++
			case "low":
				lowCount++
			}
		}

		metrics = append(metrics, models.Metric{
			Key:          "dependabot_alerts_total",
			Value:        float64(len(dependabotAlerts)),
			DisplayValue: fmt.Sprintf("%d", len(dependabotAlerts)),
			Description:  "Total open Dependabot alerts",
		})
		metrics = append(metrics, models.Metric{
			Key:          "dependabot_critical",
			Value:        float64(criticalCount),
			DisplayValue: fmt.Sprintf("%d", criticalCount),
			Description:  "Critical severity alerts",
		})
		metrics = append(metrics, models.Metric{
			Key:          "dependabot_high",
			Value:        float64(highCount),
			DisplayValue: fmt.Sprintf("%d", highCount),
			Description:  "High severity alerts",
		})

		if criticalCount > 0 {
			findings = append(findings, models.Finding{
				Type:        "critical_vulnerabilities",
				Severity:    models.SeverityHigh,
				Message:     fmt.Sprintf("%d critical vulnerability alerts found", criticalCount),
				Actionable:  true,
				Remediation: "Update vulnerable dependencies immediately.",
			})
		}
	}

	// 2. Secret Scanning Alerts (requires GHAS)
	secretAlerts, _, err := client.GetUnderlyingClient().SecretScanning.ListAlertsForRepo(ctx, repo.Owner, repo.Name, &github.SecretScanningAlertListOptions{
		State: "open",
	})

	if err == nil {
		metrics = append(metrics, models.Metric{
			Key:          "secret_scanning_alerts",
			Value:        float64(len(secretAlerts)),
			DisplayValue: fmt.Sprintf("%d", len(secretAlerts)),
			Description:  "Open secret scanning alerts",
		})

		if len(secretAlerts) > 0 {
			findings = append(findings, models.Finding{
				Type:        "leaked_secrets",
				Severity:    models.SeverityHigh,
				Message:     fmt.Sprintf("%d potential secrets found in code", len(secretAlerts)),
				Actionable:  true,
				Remediation: "Rotate leaked credentials and remove from git history.",
			})
		}
	}

	// 3. Code Scanning Alerts (requires GHAS)
	codeAlerts, _, err := client.GetUnderlyingClient().CodeScanning.ListAlertsForRepo(ctx, repo.Owner, repo.Name, &github.AlertListOptions{
		State: "open",
	})

	if err == nil {
		metrics = append(metrics, models.Metric{
			Key:          "code_scanning_alerts",
			Value:        float64(len(codeAlerts)),
			DisplayValue: fmt.Sprintf("%d", len(codeAlerts)),
			Description:  "Open code scanning alerts",
		})
	}

	// If no security features are available, add a note
	if len(metrics) == 0 {
		metrics = append(metrics, models.Metric{
			Key:          "security_features_enabled",
			Value:        0,
			DisplayValue: "No",
			Description:  "GitHub security features not available or not enabled",
		})
		findings = append(findings, models.Finding{
			Type:        "security_not_enabled",
			Severity:    models.SeverityMedium,
			Message:     "GitHub security features not detected",
			Actionable:  true,
			Remediation: "Enable Dependabot and GitHub Advanced Security.",
		})
	}

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}
