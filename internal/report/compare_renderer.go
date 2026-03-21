package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

type ComparisonTextRenderer struct{}

type metricLookup map[string]map[string]string

func (r *ComparisonTextRenderer) Render(report *models.Report, w io.Writer) error {
	return r.RenderWithOptions(report, w, RenderOptions{})
}

func (r *ComparisonTextRenderer) RenderWithOptions(report *models.Report, w io.Writer, opts RenderOptions) error {
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
	repoMetrics := make([]metricLookup, len(report.Repositories))
	for i, repo := range report.Repositories {
		repoMetrics[i] = buildMetricLookup(repo)
	}

	for _, az := range primaryRepo.Analyzers {
		// Section Header
		_, _ = fmt.Fprintf(tw, "[%s]\t", strings.ToUpper(az.Name))
		for range report.Repositories {
			_, _ = fmt.Fprint(tw, "\t")
		}
		_, _ = fmt.Fprintln(tw, "")

		for _, m := range az.Metrics {
			_, _ = fmt.Fprintf(tw, "  %s\t", m.Key)

			for _, repoLookup := range repoMetrics {
				val := "-"
				if analyzerMetrics, ok := repoLookup[az.Name]; ok {
					if metricValue, ok := analyzerMetrics[m.Key]; ok {
						val = metricValue
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

func buildMetricLookup(repo models.RepoResult) metricLookup {
	lookup := make(metricLookup, len(repo.Analyzers))
	for _, analyzer := range repo.Analyzers {
		metrics := make(map[string]string, len(analyzer.Metrics))
		for _, metric := range analyzer.Metrics {
			value := metric.DisplayValue
			if value == "" {
				value = fmt.Sprintf("%.2f", metric.Value)
			}
			metrics[metric.Key] = value
		}
		lookup[analyzer.Name] = metrics
	}
	return lookup
}
