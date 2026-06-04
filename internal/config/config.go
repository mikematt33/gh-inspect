package config

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	Global    GlobalConfig    `yaml:"global"`
	Analyzers AnalyzersConfig `yaml:"analyzers"`
	Actions   ActionsConfig   `yaml:"actions"`
}

type GlobalConfig struct {
	Concurrency  int         `yaml:"concurrency"`
	GitHubToken  string      `yaml:"github_token,omitempty"`
	GitHubTokens []string    `yaml:"github_tokens,omitempty"` // pool of PATs for rotation
	Apps         []AppConfig `yaml:"github_apps,omitempty"`   // GitHub App installations
	OutputMode   string      `yaml:"output_mode,omitempty"`   // observational (default), suggestive, statistical
}

// AppConfig holds credentials for a single GitHub App installation.
// PrivateKey may be an inline PEM string or a path to a PEM file.
type AppConfig struct {
	Name           string `yaml:"name,omitempty"`
	AppID          int64  `yaml:"app_id"`
	InstallationID int64  `yaml:"installation_id"`
	PrivateKey     string `yaml:"private_key,omitempty"`      // inline PEM or file path
	PrivateKeyPath string `yaml:"private_key_path,omitempty"` // explicit file path
}

// ActionsConfig holds defaults for the `actions` analytics command.
type ActionsConfig struct {
	Days                 int     `yaml:"days,omitempty"`
	MaxRuns              int     `yaml:"max_runs,omitempty"`
	SampleJobRuns        int     `yaml:"sample_job_runs,omitempty"`
	DurationThresholdSec float64 `yaml:"duration_threshold_sec,omitempty"`
	QueueThresholdSec    float64 `yaml:"queue_threshold_sec,omitempty"`
	ConfirmThreshold     int     `yaml:"confirm_threshold,omitempty"` // est. API calls above which confirmation is required
}

type AnalyzersConfig struct {
	PRFlow       PRFlowConfig       `yaml:"pr_flow"`
	IssueHygiene IssueHygieneConfig `yaml:"issue_hygiene"`
	RepoHealth   RepoHealthConfig   `yaml:"repo_health"`
	CI           CIConfig           `yaml:"ci"`
	Security     SecurityConfig     `yaml:"security"`
	Releases     ReleasesConfig     `yaml:"releases"`
	Branches     BranchesConfig     `yaml:"branches"`
	Dependencies DependenciesConfig `yaml:"dependencies"`
}

type PRFlowConfig struct {
	Enabled bool         `yaml:"enabled"`
	Params  PRFlowParams `yaml:"params"`
}

type PRFlowParams struct {
	StaleThresholdDays int `yaml:"stale_threshold_days"`
}

type IssueHygieneConfig struct {
	Enabled bool               `yaml:"enabled"`
	Params  IssueHygieneParams `yaml:"params"`
}

type IssueHygieneParams struct {
	StaleThresholdDays  int `yaml:"stale_threshold_days"`
	ZombieThresholdDays int `yaml:"zombie_threshold_days"`
}

type RepoHealthConfig struct {
	Enabled bool `yaml:"enabled"`
}

type CIConfig struct {
	Enabled bool `yaml:"enabled"`
}

type SecurityConfig struct {
	Enabled bool `yaml:"enabled"`
}

type ReleasesConfig struct {
	Enabled bool `yaml:"enabled"`
}

type BranchesConfig struct {
	Enabled bool         `yaml:"enabled"`
	Params  BranchParams `yaml:"params"`
}

type BranchParams struct {
	StaleThresholdDays int `yaml:"stale_threshold_days"`
}

type DependenciesConfig struct {
	Enabled bool `yaml:"enabled"`
}

func GetConfigPath() (string, error) {
	// Respect XDG_CONFIG_HOME if set (useful for testing and Linux users)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "gh-inspect", "config.yaml"), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "gh-inspect", "config.yaml"), nil
}

func Load() (*Config, error) {
	// Defaults
	cfg := &Config{
		Global: GlobalConfig{
			Concurrency: 5,
			OutputMode:  "observational", // default mode
		},
		Analyzers: AnalyzersConfig{
			PRFlow: PRFlowConfig{
				Enabled: true,
				Params: PRFlowParams{
					StaleThresholdDays: 14,
				},
			},
			IssueHygiene: IssueHygieneConfig{
				Enabled: true,
				Params: IssueHygieneParams{
					StaleThresholdDays:  60,
					ZombieThresholdDays: 365,
				},
			},
			RepoHealth: RepoHealthConfig{
				Enabled: true,
			},
			CI: CIConfig{
				Enabled: true,
			},
			Security: SecurityConfig{
				Enabled: true,
			},
			Releases: ReleasesConfig{
				Enabled: true,
			},
			Branches: BranchesConfig{
				Enabled: true,
				Params: BranchParams{
					StaleThresholdDays: 90,
				},
			},
			Dependencies: DependenciesConfig{
				Enabled: true,
			},
		},
		Actions: ActionsConfig{
			Days:                 30,
			MaxRuns:              1000,
			SampleJobRuns:        0,
			DurationThresholdSec: 1800,
			QueueThresholdSec:    300,
			ConfirmThreshold:     1000,
		},
	}

	// Try loading from file
	// Priorities: ./config.yaml, $XDG_CONFIG_HOME/gh-inspect/config.yaml, $HOME/.gh-inspect.yaml
	configDirs := []string{"config.yaml"} // Local override

	// Standard User Config Dir
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		configDirs = append(configDirs, userConfigDir+"/gh-inspect/config.yaml")
	}

	// Legacy fallback
	if home := os.Getenv("HOME"); home != "" {
		configDirs = append(configDirs, home+"/.gh-inspect.yaml")
	}

	for _, p := range configDirs {
		if _, err := os.Stat(p); err == nil {
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("error parsing %s: %w", p, err)
			}
			return cfg, nil
		}
	}

	return cfg, nil
}
