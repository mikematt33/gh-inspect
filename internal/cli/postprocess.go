package cli

import (
	"fmt"

	"github.com/mikematt33/gh-inspect/pkg/baseline"
	"github.com/mikematt33/gh-inspect/pkg/models"
	"github.com/mikematt33/gh-inspect/pkg/remediation"
)

func resolveBaselinePath() string {
	if flagBaseline != "" {
		return flagBaseline
	}
	return baseline.GetDefaultBaselinePath()
}

func resolveRemediationPath() string {
	if flagRemediationFile != "" {
		return flagRemediationFile
	}
	return remediation.GetDefaultPath()
}

func applyRemediationTracking(report *models.Report) {
	store, err := remediation.Load(resolveRemediationPath())
	if err != nil {
		if shouldPrintInfo() {
			fmt.Printf("⚠️  Could not load remediation store: %v\n", err)
		}
		return
	}
	remediation.ApplyReport(report, store)
}

func handleBaselineFeatures(report *models.Report) (*baseline.ComparisonResult, *baseline.TrendSummary, error) {
	baselinePath := resolveBaselinePath()

	var comparison *baseline.ComparisonResult
	if flagCompareLast || flagBaseline != "" {
		previousBaseline, err := baseline.Load(baselinePath)
		if err != nil {
			if shouldPrintInfo() {
				fmt.Printf("⚠️  Could not load baseline for comparison: %v\n", err)
			}
		} else {
			comparison = baseline.Compare(report, previousBaseline)
			if shouldPrintInfo() && comparison != nil {
				printComparison(comparison)
			}
		}
	}

	var trend *baseline.TrendSummary
	if flagCompareHistory > 0 {
		history, err := baseline.LoadHistory(baselinePath, flagCompareHistory)
		if err != nil {
			if shouldPrintInfo() {
				fmt.Printf("⚠️  Could not load baseline history: %v\n", err)
			}
		} else {
			trend = baseline.ComputeTrend(report, history)
			if shouldPrintInfo() && trend != nil {
				printTrendSummary(trend)
			}
		}
	}

	if flagSaveBaseline {
		snapshotPath, err := baseline.SaveWithHistory(report, baselinePath)
		if err != nil {
			if shouldPrintInfo() {
				fmt.Printf("⚠️  Failed to save baseline: %v\n", err)
			}
		} else if shouldPrintInfo() {
			fmt.Printf("\n✅ Baseline saved to %s\n", baselinePath)
			fmt.Printf("✅ Snapshot saved to %s\n", snapshotPath)
		}
	}

	if flagFailOnRegression && comparison != nil && comparison.Summary.HasRegression {
		return comparison, trend, fmt.Errorf("regression detected compared to baseline")
	}

	return comparison, trend, nil
}
