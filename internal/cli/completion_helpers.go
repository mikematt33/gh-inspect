package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/spf13/cobra"
)

// completion_helpers.go provides dynamic shell completion support for gh-inspect.
//
// Features:
// - Tracks recently used repositories, organizations, and users
// - Suggests recent items during tab completion
// - Fetches live data from GitHub API when authenticated
// - Stores completion history in ~/.config/gh-inspect/completion-history.json
//
// The completion functions (completeRepositories, completeOrganizations, completeUsers)
// are registered with Cobra commands via ValidArgsFunction and provide intelligent
// suggestions based on usage patterns and GitHub API data.

// recentItem tracks recently used items for completions
type recentItem struct {
	Value     string    `json:"value"`
	LastUsed  time.Time `json:"last_used"`
	UseCount  int       `json:"use_count"`
	ItemType  string    `json:"type"` // "repo", "org", "user"
}

type recentHistory struct {
	Items []recentItem `json:"items"`
}

// getHistoryPath returns the path to the completion history file
func getHistoryPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "gh-inspect", "completion-history.json"), nil
}

// loadHistory loads recent completion history
func loadHistory() (*recentHistory, error) {
	path, err := getHistoryPath()
	if err != nil {
		return &recentHistory{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &recentHistory{}, nil
		}
		return nil, err
	}

	var history recentHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return &recentHistory{}, nil // Return empty on parse error
	}

	return &history, nil
}

// saveHistory saves completion history
func saveHistory(history *recentHistory) error {
	path, err := getHistoryPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Limit history size to 100 most recent items
	if len(history.Items) > 100 {
		// Sort by last used time
		sort.Slice(history.Items, func(i, j int) bool {
			return history.Items[i].LastUsed.After(history.Items[j].LastUsed)
		})
		history.Items = history.Items[:100]
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// recordUsage records that an item was used
func recordUsage(value, itemType string) {
	history, err := loadHistory()
	if err != nil {
		return // Silently fail on history tracking errors
	}

	// Find existing or create new
	found := false
	for i := range history.Items {
		if history.Items[i].Value == value && history.Items[i].ItemType == itemType {
			history.Items[i].LastUsed = time.Now()
			history.Items[i].UseCount++
			found = true
			break
		}
	}

	if !found {
		history.Items = append(history.Items, recentItem{
			Value:    value,
			LastUsed: time.Now(),
			UseCount: 1,
			ItemType: itemType,
		})
	}

	saveHistory(history)
}

// getRecentItems returns recent items of a specific type, sorted by frequency and recency
func getRecentItems(itemType string, limit int) []string {
	history, err := loadHistory()
	if err != nil {
		return nil
	}

	var filtered []recentItem
	for _, item := range history.Items {
		if item.ItemType == itemType {
			filtered = append(filtered, item)
		}
	}

	// Sort by use count (descending) then by last used (descending)
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].UseCount != filtered[j].UseCount {
			return filtered[i].UseCount > filtered[j].UseCount
		}
		return filtered[i].LastUsed.After(filtered[j].LastUsed)
	})

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	result := make([]string, len(filtered))
	for i, item := range filtered {
		result[i] = item.Value
	}

	return result
}

// completeRepositories provides completion for repository arguments
func completeRepositories(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Return recent repositories
	recent := getRecentItems("repo", 20)
	
	// Filter by prefix if provided
	if toComplete != "" {
		var filtered []string
		for _, r := range recent {
			if strings.HasPrefix(r, toComplete) {
				filtered = append(filtered, r)
			}
		}
		recent = filtered
	}

	// Add hint for format
	if len(recent) == 0 {
		return []string{"owner/repo"}, cobra.ShellCompDirectiveNoFileComp
	}

	return recent, cobra.ShellCompDirectiveNoFileComp
}

// completeOrganizations provides completion for organization arguments
func completeOrganizations(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Try to get organizations from authenticated user if token is available
	suggestions := []string{}

	// First, add recent organizations from history
	recent := getRecentItems("org", 10)
	suggestions = append(suggestions, recent...)

	// Try to fetch from GitHub if authenticated
	cfg, err := config.Load()
	if err == nil && cfg.Global.GitHubToken != "" {
		client, err := getClientWithToken(cfg)
		if err == nil {
			// Get user's organizations
			orgs, _, err := client.Client.Organizations.List(context.Background(), "", nil)
			if err == nil {
				for _, org := range orgs {
					if org.Login != nil {
						orgName := *org.Login
						// Avoid duplicates
						found := false
						for _, existing := range suggestions {
							if existing == orgName {
								found = true
								break
							}
						}
						if !found {
							suggestions = append(suggestions, orgName)
						}
					}
				}
			}
		}
	}

	// Filter by prefix
	if toComplete != "" {
		var filtered []string
		for _, s := range suggestions {
			if strings.HasPrefix(s, toComplete) {
				filtered = append(filtered, s)
			}
		}
		suggestions = filtered
	}

	if len(suggestions) == 0 {
		return []string{"organization-name"}, cobra.ShellCompDirectiveNoFileComp
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeUsers provides completion for user arguments
func completeUsers(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Return recent users and authenticated user
	suggestions := []string{}

	// Add recent users from history
	recent := getRecentItems("user", 10)
	suggestions = append(suggestions, recent...)

	// Try to get authenticated user
	cfg, err := config.Load()
	if err == nil && cfg.Global.GitHubToken != "" {
		client, err := getClientWithToken(cfg)
		if err == nil {
			user, _, err := client.Client.Users.Get(context.Background(), "")
			if err == nil && user.Login != nil {
				// Add authenticated user at the beginning
				suggestions = append([]string{*user.Login}, suggestions...)
			}
		}
	}

	// Filter by prefix
	if toComplete != "" {
		var filtered []string
		for _, s := range suggestions {
			if strings.HasPrefix(s, toComplete) {
				filtered = append(filtered, s)
			}
		}
		suggestions = filtered
	}

	if len(suggestions) == 0 {
		return []string{"username"}, cobra.ShellCompDirectiveNoFileComp
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}
