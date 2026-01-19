package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

type ComparisonTextRenderer struct{}

func (r *ComparisonTextRenderer) Render(report *models.Report, w io.Writer) error {
	if len(report.Repositories) == 0 {
		_, _ = fmt.Fprintln(w, "No repositories to compare.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)

	// HEADERS
	// First column empty for Metric Name
	_, _ = fmt.Fprint(tw, "METRIC\t")
	for _, repo := range report.Repositories {
		// Truncate if too long?
		name := repo.Name
		if len(name) > 20 {
			name = "..." + name[len(name)-17:]
		}
		_, _ = fmt.Fprintf(tw, "%s\t", name)
	}
	_, _ = fmt.Fprintln(tw, "")

	// Separator
	_, _ = fmt.Fprint(tw, "------\t")
	for range report.Repositories {
		_, _ = fmt.Fprint(tw, "------\t")
	}
	_, _ = fmt.Fprintln(tw, "")

	// DATA ROWS
	// robust way: collect all unique (Analyzer, MetricKey) pairs
	// simple way: assume all repos have same analyzers/metrics orders (mostly true for this CLI)
	// We'll use the first repo as the template for rows
	primaryRepo := report.Repositories[0]

	for _, az := range primaryRepo.Analyzers {
		// Section Header
		_, _ = fmt.Fprintf(tw, "[%s]\t", strings.ToUpper(az.Name))
		for range report.Repositories {
			_, _ = fmt.Fprint(tw, "\t")
		}
		_, _ = fmt.Fprintln(tw, "")

		for _, m := range az.Metrics {
			_, _ = fmt.Fprintf(tw, "  %s\t", m.Key)

			// For each repo, find this metric
			for _, repo := range report.Repositories {
				val := "-"
				// specific analyzer search
				var targetAz *models.AnalyzerResult
				for _, rAz := range repo.Analyzers {
					if rAz.Name == az.Name {
						targetAz = &rAz
						break
					}
				}

				if targetAz != nil {
					for _, tm := range targetAz.Metrics {
						if tm.Key == m.Key {
							val = tm.DisplayValue
							if val == "" {
								val = fmt.Sprintf("%.2f", tm.Value)
							}
							break
						}
					}
				}
				_, _ = fmt.Fprintf(tw, "%s\t", val)
			}
			_, _ = fmt.Fprintln(tw, "")
		}
		// Empty line between sections
		_, _ = fmt.Fprintln(tw, "\t")
	}

	return tw.Flush()
}
