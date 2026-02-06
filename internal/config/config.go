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
	OutputMode  string `yaml:"output_mode,omitempty"` // observational (default), suggestive, statistical
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
		return xdgConfig + "/gh-inspect/config.yaml", nil
	}

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

// Save writes the configuration to the user's config file
func Save(cfg *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return fmt.Errorf("error getting config path: %w", err)
	}

	// Ensure the directory exists
	configDir := configPath[:len(configPath)-len("/config.yaml")]
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	// Marshal the config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
