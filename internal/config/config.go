package config

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	Global    GlobalConfig    `yaml:"global"`
	Analyzers AnalyzersConfig `yaml:"analyzers"`
}

type GlobalConfig struct {
	Concurrency int    `yaml:"concurrency"`
	GitHubToken string `yaml:"github_token,omitempty"`
}

type AnalyzersConfig struct {
	PRFlow       PRFlowConfig       `yaml:"pr_flow"`
	IssueHygiene IssueHygieneConfig `yaml:"issue_hygiene"`
	RepoHealth   RepoHealthConfig   `yaml:"repo_health"`
	CI           CIConfig           `yaml:"ci"`
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

func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return configDir + "/gh-inspect/config.yaml", nil
}

func Load() (*Config, error) {
	// Defaults
	cfg := &Config{
		Global: GlobalConfig{
			Concurrency: 5,
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
					StaleThresholdDays:  30,
					ZombieThresholdDays: 180,
				},
			},
			RepoHealth: RepoHealthConfig{
				Enabled: true,
			},
			CI: CIConfig{
				Enabled: true,
			},
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
