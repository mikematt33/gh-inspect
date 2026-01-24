package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
)

// parseDuration parses a duration string like "30d" or "720h"
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		var days int
		_, err := fmt.Sscanf(daysStr, "%d", &days)
		if err != nil {
			return 0, fmt.Errorf("invalid day format: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// RepoFilter applies filtering logic to repositories
type RepoFilter struct {
	NamePattern   *regexp.Regexp
	Languages     []string
	Topics        []string
	UpdatedWithin time.Duration
	SkipForks     bool
}

// NewRepoFilter creates a filter from CLI flags
func NewRepoFilter() (*RepoFilter, error) {
	filter := &RepoFilter{
		Languages: flagFilterLanguage,
		Topics:    flagFilterTopics,
		SkipForks: flagFilterSkipForks,
	}

	// Compile name regex if provided
	if flagFilterName != "" {
		pattern, err := regexp.Compile(flagFilterName)
		if err != nil {
			return nil, err
		}
		filter.NamePattern = pattern
	}

	// Parse updated duration if provided
	if flagFilterUpdated != "" {
		duration, err := parseDuration(flagFilterUpdated)
		if err != nil {
			return nil, err
		}
		filter.UpdatedWithin = duration
	}

	return filter, nil
}

// Matches returns true if the repository passes all filter criteria
func (f *RepoFilter) Matches(repo *github.Repository) bool {
	// Skip archived repositories (always)
	if repo.GetArchived() {
		return false
	}

	// Skip forks if requested
	if f.SkipForks && repo.GetFork() {
		return false
	}

	// Name pattern filter
	if f.NamePattern != nil {
		if !f.NamePattern.MatchString(repo.GetName()) {
			return false
		}
	}

	// Language filter
	if len(f.Languages) > 0 {
		repoLang := strings.ToLower(repo.GetLanguage())
		matched := false
		for _, lang := range f.Languages {
			if strings.ToLower(lang) == repoLang {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Topics filter (repository must have ALL specified topics)
	if len(f.Topics) > 0 {
		repoTopics := repo.Topics
		for _, requiredTopic := range f.Topics {
			found := false
			for _, repoTopic := range repoTopics {
				if strings.EqualFold(requiredTopic, repoTopic) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Updated within filter
	if f.UpdatedWithin > 0 {
		updatedAt := repo.GetUpdatedAt()
		if updatedAt.Before(time.Now().Add(-f.UpdatedWithin)) {
			return false
		}
	}

	return true
}

// Stats tracks filtering statistics
type FilterStats struct {
	Total         int
	Archived      int
	Forks         int
	NameFiltered  int
	LangFiltered  int
	TopicFiltered int
	DateFiltered  int
	Passed        int
}

// FilterRepositories applies filters and returns matching repository names with statistics
func FilterRepositories(repos []*github.Repository, filter *RepoFilter) ([]string, *FilterStats) {
	stats := &FilterStats{
		Total: len(repos),
	}

	var targetRepos []string

	for _, r := range repos {
		// Track archived separately
		if r.GetArchived() {
			stats.Archived++
			continue
		}

		// Track forks
		if r.GetFork() {
			stats.Forks++
			if filter.SkipForks {
				continue
			}
		}

		// Apply remaining filters
		passed := true

		// Name filter
		if filter.NamePattern != nil && !filter.NamePattern.MatchString(r.GetName()) {
			stats.NameFiltered++
			passed = false
		}

		// Language filter
		if passed && len(filter.Languages) > 0 {
			repoLang := strings.ToLower(r.GetLanguage())
			matched := false
			for _, lang := range filter.Languages {
				if strings.ToLower(lang) == repoLang {
					matched = true
					break
				}
			}
			if !matched {
				stats.LangFiltered++
				passed = false
			}
		}

		// Topics filter
		if passed && len(filter.Topics) > 0 {
			repoTopics := r.Topics
			for _, requiredTopic := range filter.Topics {
				found := false
				for _, repoTopic := range repoTopics {
					if strings.EqualFold(requiredTopic, repoTopic) {
						found = true
						break
					}
				}
				if !found {
					stats.TopicFiltered++
					passed = false
					break
				}
			}
		}

		// Date filter
		if passed && filter.UpdatedWithin > 0 {
			updatedAt := r.GetUpdatedAt()
			if updatedAt.Before(time.Now().Add(-filter.UpdatedWithin)) {
				stats.DateFiltered++
				passed = false
			}
		}

		if passed {
			stats.Passed++
			targetRepos = append(targetRepos, r.GetFullName())
		}
	}

	return targetRepos, stats
}
