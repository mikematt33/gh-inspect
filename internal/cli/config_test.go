package cli

import (
	"testing"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestSetConfigValue(t *testing.T) {
	// Initialize a complex config object to test against
	cfg := &config.Config{
		Global: config.GlobalConfig{
			GitHubToken: "old-token",
			Concurrency: 5,
		},
		Analyzers: config.AnalyzersConfig{
			PRFlow: config.PRFlowConfig{
				Enabled: true,
				Params: config.PRFlowParams{
					StaleThresholdDays: 10,
				},
			},
			IssueHygiene: config.IssueHygieneConfig{
				Params: config.IssueHygieneParams{
					ZombieThresholdDays: 180,
				},
			},
		},
	}

	tests := []struct {
		name      string
		key       string
		val       string
		wantErr   bool
		validator func(*config.Config) bool
	}{
		{
			name: "Set Global String",
			key:  "global.github_token",
			val:  "new-token",
			validator: func(c *config.Config) bool {
				return c.Global.GitHubToken == "new-token"
			},
		},
		{
			name: "Set Global Int",
			key:  "global.concurrency",
			val:  "20",
			validator: func(c *config.Config) bool {
				return c.Global.Concurrency == 20
			},
		},
		{
			name: "Set Nested Bool",
			key:  "analyzers.pr_flow.enabled",
			val:  "false",
			validator: func(c *config.Config) bool {
				return c.Analyzers.PRFlow.Enabled == false
			},
		},
		{
			name: "Set Deep Nested Int",
			key:  "analyzers.pr_flow.params.stale_threshold_days",
			val:  "99",
			validator: func(c *config.Config) bool {
				return c.Analyzers.PRFlow.Params.StaleThresholdDays == 99
			},
		},
		{
			name: "Set Another Deep Nested Int",
			key:  "analyzers.issue_hygiene.params.zombie_threshold_days",
			val:  "365",
			validator: func(c *config.Config) bool {
				return c.Analyzers.IssueHygiene.Params.ZombieThresholdDays == 365
			},
		},
		{
			name:    "Invalid Key",
			key:     "global.unknown_field",
			val:     "foo",
			wantErr: true,
		},
		{
			name:    "Invalid Type Match (Int expected)",
			key:     "global.concurrency",
			val:     "not-an-int",
			wantErr: true,
		},
		{
			name:    "Invalid Type Match (Bool expected)",
			key:     "analyzers.pr_flow.enabled",
			val:     "maybe",
			wantErr: true,
		},
		{
			name:    "Part is not a struct",
			key:     "global.concurrency.subfield",
			val:     "10",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := setConfigValue(cfg, tt.key, tt.val)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validator != nil {
					assert.True(t, tt.validator(cfg))
				}
			}
		})
	}
}
