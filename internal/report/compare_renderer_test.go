package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

func TestComparisonTextRendererRendersMetricMatrix(t *testing.T) {
	report := &models.Report{
		Repositories: []models.RepoResult{
			{
				Name: "owner/repo-one",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: 95, DisplayValue: "95%"},
							{Key: "avg_runtime", Value: 123.456},
						},
					},
				},
			},
			{
				Name: "owner/repo-two",
				Analyzers: []models.AnalyzerResult{
					{
						Name: "ci",
						Metrics: []models.Metric{
							{Key: "success_rate", Value: 88, DisplayValue: "88%"},
							{Key: "avg_runtime", Value: 42},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := (&ComparisonTextRenderer{}).RenderWithOptions(report, &buf, RenderOptions{})
	if err != nil {
		t.Fatalf("RenderWithOptions failed: %v", err)
	}

	output := buf.String()
	for _, expected := range []string{"[CI]", "success_rate", "95%", "88%", "123.46", "42.00"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}