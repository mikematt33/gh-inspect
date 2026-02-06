package cli

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage the gh-inspect configuration file.
The configuration file is typically located at:
- Linux: ~/.config/gh-inspect/config.yaml
- macOS: ~/Library/Application Support/gh-inspect/config.yaml
- Windows: %APPDATA%\gh-inspect\config.yaml`,
}

var setTokenCmd = &cobra.Command{
	Use:   "set-token [token]",
	Short: "Shortcut to set the GitHub API token",
	Args:  cobra.ExactArgs(1),
	Run:   runSetToken,
}

var setCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Long: `Set a configuration value using dot notation.
Examples:
  gh-inspect config set global.concurrency 10
  gh-inspect config set analyzers.pr_flow.enabled false
  gh-inspect config set analyzers.issue_hygiene.params.zombie_threshold_days 90`,
	Args: cobra.ExactArgs(2),
	Run:  runSet,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List current configuration",
	Run:   runList,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setTokenCmd)
	configCmd.AddCommand(setCmd)

	setCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{
			"global.concurrency",
			"global.github_token",
			"global.output_mode",
			"analyzers.pr_flow.enabled",
			"analyzers.pr_flow.params.stale_threshold_days",
			"analyzers.pr_flow.params.cycle_time_target_hours",
			"analyzers.issue_hygiene.enabled",
			"analyzers.issue_hygiene.params.stale_threshold_days",
			"analyzers.issue_hygiene.params.zombie_threshold_days",
			"analyzers.repo_health.enabled",
			"analyzers.ci.enabled",
		}, cobra.ShellCompDirectiveNoFileComp
	}

	configCmd.AddCommand(listCmd)
}

func saveConfig(cfg *config.Config) error {
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("error resolving config path: %w", err)
	}

	configDir := configPath[:len(configPath)-len("/config.yaml")]
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	fmt.Printf("✅ Configuration saved to %s\n", configPath)
	return nil
}

func runSetToken(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	cfg.Global.GitHubToken = args[0]
	if err := saveConfig(cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runList(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Just dump the yaml
	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Printf("Error marshaling config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func runSet(cmd *cobra.Command, args []string) {
	key := args[0]
	valStr := args[1]

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := setConfigValue(cfg, key, valStr); err != nil {
		fmt.Printf("Error setting value: %v\n", err)
		os.Exit(1)
	}

	if err := saveConfig(cfg); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// setConfigValue traverses the struct using reflection and sets the value
func setConfigValue(obj interface{}, path string, valStr string) error {
	parts := strings.Split(path, ".")
	v := reflect.ValueOf(obj)

	// Ensure we have a pointer if we want to set it, or unwrap if it's an interface
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for i, part := range parts {
		if v.Kind() != reflect.Struct {
			return fmt.Errorf("field %s is not a struct", strings.Join(parts[:i], "."))
		}

		// Find field by yaml tag
		typ := v.Type()
		var fieldVal reflect.Value
		found := false

		for j := 0; j < typ.NumField(); j++ {
			field := typ.Field(j)
			tag := field.Tag.Get("yaml")
			cleanTag := strings.Split(tag, ",")[0]
			if cleanTag == part {
				fieldVal = v.Field(j)
				found = true
				break
			}
		}

		if !found {
			// Fallback: try case-insensitive field name match
			fieldVal = v.FieldByNameFunc(func(n string) bool {
				return strings.EqualFold(n, part)
			})
			if !fieldVal.IsValid() {
				return fmt.Errorf("field '%s' not found", part)
			}
		}

		v = fieldVal
	}

	if !v.CanSet() {
		return fmt.Errorf("cannot set field %s", path)
	}

	// Set value based on type
	switch v.Kind() {
	case reflect.String:
		v.SetString(valStr)
	case reflect.Int, reflect.Int64:
		i, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", valStr)
		}
		v.SetInt(i)
	case reflect.Bool:
		b, err := strconv.ParseBool(valStr)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", valStr)
		}
		v.SetBool(b)
	default:
		return fmt.Errorf("unsupported type %s for key %s", v.Kind(), path)
	}

	return nil
}
