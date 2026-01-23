package analysis

import (
	"testing"
)

func TestGetDepthConfig(t *testing.T) {
	tests := []struct {
		name            string
		depth           string
		wantIncludeDeep bool
		wantMaxPRs      int
		wantMaxIssues   int
		wantMaxRuns     int
	}{
		{
			name:            "shallow",
			depth:           "shallow",
			wantIncludeDeep: false,
			wantMaxPRs:      50,
			wantMaxIssues:   100,
			wantMaxRuns:     50,
		},
		{
			name:            "standard",
			depth:           "standard",
			wantIncludeDeep: false,
			wantMaxPRs:      100,
			wantMaxIssues:   200,
			wantMaxRuns:     100,
		},
		{
			name:            "deep",
			depth:           "deep",
			wantIncludeDeep: true,
			wantMaxPRs:      500,
			wantMaxIssues:   1000,
			wantMaxRuns:     500,
		},
		{
			name:            "invalid defaults to standard",
			depth:           "invalid",
			wantIncludeDeep: false,
			wantMaxPRs:      100,
			wantMaxIssues:   200,
			wantMaxRuns:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDepthConfig(tt.depth)
			if got.IncludeDeep != tt.wantIncludeDeep {
				t.Errorf("GetDepthConfig(%q).IncludeDeep = %v, want %v", tt.depth, got.IncludeDeep, tt.wantIncludeDeep)
			}
			if got.MaxPRs != tt.wantMaxPRs {
				t.Errorf("GetDepthConfig(%q).MaxPRs = %v, want %v", tt.depth, got.MaxPRs, tt.wantMaxPRs)
			}
			if got.MaxIssues != tt.wantMaxIssues {
				t.Errorf("GetDepthConfig(%q).MaxIssues = %v, want %v", tt.depth, got.MaxIssues, tt.wantMaxIssues)
			}
			if got.MaxWorkflowRuns != tt.wantMaxRuns {
				t.Errorf("GetDepthConfig(%q).MaxWorkflowRuns = %v, want %v", tt.depth, got.MaxWorkflowRuns, tt.wantMaxRuns)
			}
		})
	}
}

func TestApplyOverrides(t *testing.T) {
	tests := []struct {
		name            string
		base            DepthConfig
		maxPRs          int
		maxIssues       int
		maxWorkflowRuns int
		wantMaxPRs      int
		wantMaxIssues   int
		wantMaxRuns     int
	}{
		{
			name:            "no overrides",
			base:            StandardDepth,
			maxPRs:          0,
			maxIssues:       0,
			maxWorkflowRuns: 0,
			wantMaxPRs:      100,
			wantMaxIssues:   200,
			wantMaxRuns:     100,
		},
		{
			name:            "override PRs only",
			base:            StandardDepth,
			maxPRs:          25,
			maxIssues:       0,
			maxWorkflowRuns: 0,
			wantMaxPRs:      25,
			wantMaxIssues:   200,
			wantMaxRuns:     100,
		},
		{
			name:            "override all",
			base:            ShallowDepth,
			maxPRs:          10,
			maxIssues:       20,
			maxWorkflowRuns: 15,
			wantMaxPRs:      10,
			wantMaxIssues:   20,
			wantMaxRuns:     15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.ApplyOverrides(tt.maxPRs, tt.maxIssues, tt.maxWorkflowRuns)
			if got.MaxPRs != tt.wantMaxPRs {
				t.Errorf("ApplyOverrides().MaxPRs = %v, want %v", got.MaxPRs, tt.wantMaxPRs)
			}
			if got.MaxIssues != tt.wantMaxIssues {
				t.Errorf("ApplyOverrides().MaxIssues = %v, want %v", got.MaxIssues, tt.wantMaxIssues)
			}
			if got.MaxWorkflowRuns != tt.wantMaxRuns {
				t.Errorf("ApplyOverrides().MaxWorkflowRuns = %v, want %v", got.MaxWorkflowRuns, tt.wantMaxRuns)
			}
		})
	}
}
