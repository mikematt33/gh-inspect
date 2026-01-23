package cli

import (
	"fmt"
	"time"

	"github.com/mikematt33/gh-inspect/pkg/baseline"
	"github.com/mikematt33/gh-inspect/pkg/util"
)

// printComparison prints a comparison result in a human-readable format
func printComparison(comp *baseline.ComparisonResult) {
	fmt.Println("\n" + colorBold + "📊 Comparison with Baseline" + colorReset)
	fmt.Printf("Previous run: %s\n", comp.Previous.Timestamp.Format(time.RFC3339))
	fmt.Println()

	summary := comp.Summary

	// Overall Status
	if summary.HasRegression {
		fmt.Println(colorRed + "⚠️  REGRESSION DETECTED" + colorReset)
	} else {
		fmt.Println(colorGreen + "✅ No significant regression" + colorReset)
	}
	fmt.Println()

	// Key Metrics
	fmt.Println(colorBold + "Key Changes:" + colorReset)
	printMetricDelta("Health Score", summary.HealthScoreDelta, true)
	printMetricDelta("CI Success Rate", summary.CISuccessRateDelta, true)
	printMetricDelta("PR Cycle Time", summary.PRCycleTimeDelta, false)
	printMetricDelta("Zombie Issues", float64(summary.ZombieIssueDelta), false)
	fmt.Println()

	// Metrics Summary
	fmt.Printf("📈 Improved metrics: %s%d%s\n", colorGreen, summary.TotalImprovedMetrics, colorReset)
	fmt.Printf("📉 Degraded metrics: %s%d%s\n", colorRed, summary.TotalDegradedMetrics, colorReset)
	fmt.Println()

	// Detailed Changes (show top 5 improvements and degradations)
	showTopChanges(comp)
}

// printMetricDelta prints a metric change with color coding
func printMetricDelta(name string, delta float64, higherIsBetter bool) {
	arrow := "→"
	color := colorReset

	if delta > 0 {
		arrow = "↑"
		if higherIsBetter {
			color = colorGreen
		} else {
			color = colorRed
		}
	} else if delta < 0 {
		arrow = "↓"
		if higherIsBetter {
			color = colorRed
		} else {
			color = colorGreen
		}
	}

	fmt.Printf("  %-20s %s%s %.2f%s\n", name+":", color, arrow, delta, colorReset)
}

// showTopChanges displays the most significant metric changes
func showTopChanges(comp *baseline.ComparisonResult) {
	if len(comp.Deltas) == 0 {
		return
	}

	// Collect all changes
	var improvements []baseline.MetricChange
	var degradations []baseline.MetricChange

	for _, repoDelta := range comp.Deltas {
		for _, change := range repoDelta.MetricDiff {
			if change.Improved {
				improvements = append(improvements, change)
			} else {
				degradations = append(degradations, change)
			}
		}
	}

	// Show top improvements
	if len(improvements) > 0 {
		fmt.Println(colorGreen + "Top Improvements:" + colorReset)
		count := util.Min(5, len(improvements))
		for i := 0; i < count; i++ {
			change := improvements[i]
			fmt.Printf("  • %s: %.2f → %.2f (%.1f%%)\n",
				change.Key, change.Previous, change.Current, change.PercentDelta)
		}
		fmt.Println()
	}

	// Show top degradations
	if len(degradations) > 0 {
		fmt.Println(colorRed + "Top Degradations:" + colorReset)
		count := util.Min(5, len(degradations))
		for i := 0; i < count; i++ {
			change := degradations[i]
			fmt.Printf("  • %s: %.2f → %.2f (%.1f%%)\n",
				change.Key, change.Previous, change.Current, change.PercentDelta)
		}
		fmt.Println()
	}
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)
