package cli

import (
	"regexp"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "days format",
			input:    "30d",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "single day",
			input:    "1d",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "hours format",
			input:    "72h",
			expected: 72 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "minutes format",
			input:    "30m",
			expected: 30 * time.Minute,
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid days format",
			input:   "xd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func createTestRepo(name, language string, topics []string, archived, fork bool, updatedAt time.Time) *github.Repository {
	return &github.Repository{
		Name:      github.String(name),
		FullName:  github.String("owner/" + name),
		Language:  github.String(language),
		Topics:    topics,
		Archived:  github.Bool(archived),
		Fork:      github.Bool(fork),
		UpdatedAt: &github.Timestamp{Time: updatedAt},
	}
}

func TestRepoFilterMatches(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		filter         *RepoFilter
		repo           *github.Repository
		expectedMatch  bool
	}{
		{
			name:   "no filters - should pass",
			filter: &RepoFilter{},
			repo:   createTestRepo("test-repo", "Go", []string{}, false, false, now),
			expectedMatch: true,
		},
		{
			name:   "archived repo - should fail",
			filter: &RepoFilter{},
			repo:   createTestRepo("archived-repo", "Go", []string{}, true, false, now),
			expectedMatch: false,
		},
		{
			name:   "fork with skip forks - should fail",
			filter: &RepoFilter{SkipForks: true},
			repo:   createTestRepo("forked-repo", "Go", []string{}, false, true, now),
			expectedMatch: false,
		},
		{
			name:   "fork without skip forks - should pass",
			filter: &RepoFilter{SkipForks: false},
			repo:   createTestRepo("forked-repo", "Go", []string{}, false, true, now),
			expectedMatch: true,
		},
		{
			name: "name pattern match",
			filter: &RepoFilter{
				NamePattern: regexp.MustCompile("^test-"),
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "name pattern no match",
			filter: &RepoFilter{
				NamePattern: regexp.MustCompile("^test-"),
			},
			repo:          createTestRepo("other-repo", "Go", []string{}, false, false, now),
			expectedMatch: false,
		},
		{
			name: "language filter match",
			filter: &RepoFilter{
				Languages: []string{"Go"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "language filter case insensitive",
			filter: &RepoFilter{
				Languages: []string{"go"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "language filter no match",
			filter: &RepoFilter{
				Languages: []string{"Python"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now),
			expectedMatch: false,
		},
		{
			name: "multiple languages - one match",
			filter: &RepoFilter{
				Languages: []string{"Python", "Go", "Java"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "topics filter - all present",
			filter: &RepoFilter{
				Topics: []string{"cli", "golang"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{"cli", "golang", "tool"}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "topics filter - missing one",
			filter: &RepoFilter{
				Topics: []string{"cli", "python"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{"cli", "golang"}, false, false, now),
			expectedMatch: false,
		},
		{
			name: "topics filter - case insensitive",
			filter: &RepoFilter{
				Topics: []string{"CLI"},
			},
			repo:          createTestRepo("test-repo", "Go", []string{"cli"}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "updated within - recent",
			filter: &RepoFilter{
				UpdatedWithin: 24 * time.Hour,
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now.Add(-1*time.Hour)),
			expectedMatch: true,
		},
		{
			name: "updated within - too old",
			filter: &RepoFilter{
				UpdatedWithin: 24 * time.Hour,
			},
			repo:          createTestRepo("test-repo", "Go", []string{}, false, false, now.Add(-48*time.Hour)),
			expectedMatch: false,
		},
		{
			name: "multiple filters combined - pass",
			filter: &RepoFilter{
				NamePattern:   regexp.MustCompile("^test-"),
				Languages:     []string{"Go"},
				Topics:        []string{"cli"},
				UpdatedWithin: 24 * time.Hour,
				SkipForks:     false,
			},
			repo:          createTestRepo("test-repo", "Go", []string{"cli"}, false, false, now),
			expectedMatch: true,
		},
		{
			name: "multiple filters combined - fail one",
			filter: &RepoFilter{
				NamePattern:   regexp.MustCompile("^test-"),
				Languages:     []string{"Go"},
				Topics:        []string{"python"},
				UpdatedWithin: 24 * time.Hour,
			},
			repo:          createTestRepo("test-repo", "Go", []string{"cli"}, false, false, now),
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Matches(tt.repo)
			if result != tt.expectedMatch {
				t.Errorf("Expected %v, got %v", tt.expectedMatch, result)
			}
		})
	}
}

func TestFilterRepositories(t *testing.T) {
	now := time.Now()

	repos := []*github.Repository{
		createTestRepo("active-go-repo", "Go", []string{"cli"}, false, false, now),
		createTestRepo("active-python-repo", "Python", []string{"web"}, false, false, now),
		createTestRepo("archived-repo", "Go", []string{}, true, false, now),
		createTestRepo("forked-repo", "Go", []string{}, false, true, now),
		createTestRepo("old-repo", "Go", []string{}, false, false, now.Add(-100*24*time.Hour)),
		createTestRepo("test-go-cli", "Go", []string{"cli", "tool"}, false, false, now),
	}

	t.Run("no filters", func(t *testing.T) {
		filter := &RepoFilter{}
		results, stats := FilterRepositories(repos, filter)

		if stats.Total != 6 {
			t.Errorf("Expected total 6, got %d", stats.Total)
		}
		if stats.Archived != 1 {
			t.Errorf("Expected 1 archived, got %d", stats.Archived)
		}
		if stats.Forks != 1 {
			t.Errorf("Expected 1 fork, got %d", stats.Forks)
		}
		if stats.Passed != 5 {
			t.Errorf("Expected 5 passed, got %d", stats.Passed)
		}
		if len(results) != 5 {
			t.Errorf("Expected 5 results, got %d", len(results))
		}
	})

	t.Run("skip forks", func(t *testing.T) {
		filter := &RepoFilter{SkipForks: true}
		results, stats := FilterRepositories(repos, filter)

		if stats.Forks != 1 {
			t.Errorf("Expected 1 fork, got %d", stats.Forks)
		}
		if stats.Passed != 4 {
			t.Errorf("Expected 4 passed (archived excluded, fork skipped), got %d", stats.Passed)
		}
		if len(results) != 4 {
			t.Errorf("Expected 4 results, got %d", len(results))
		}
	})

	t.Run("language filter", func(t *testing.T) {
		filter := &RepoFilter{Languages: []string{"Go"}}
		results, stats := FilterRepositories(repos, filter)

		if stats.LangFiltered != 1 {
			t.Errorf("Expected 1 language filtered, got %d", stats.LangFiltered)
		}
		// Should get 4 Go repos (excluding archived)
		if stats.Passed != 4 {
			t.Errorf("Expected 4 passed, got %d", stats.Passed)
		}
		if len(results) != 4 {
			t.Errorf("Expected 4 results, got %d", len(results))
		}
	})

	t.Run("name pattern filter", func(t *testing.T) {
		filter := &RepoFilter{NamePattern: regexp.MustCompile("^test-")}
		results, stats := FilterRepositories(repos, filter)

		if stats.NameFiltered != 4 {
			t.Errorf("Expected 4 name filtered, got %d", stats.NameFiltered)
		}
		if stats.Passed != 1 {
			t.Errorf("Expected 1 passed, got %d", stats.Passed)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
		if len(results) > 0 && results[0] != "owner/test-go-cli" {
			t.Errorf("Expected 'owner/test-go-cli', got '%s'", results[0])
		}
	})

	t.Run("topics filter", func(t *testing.T) {
		filter := &RepoFilter{Topics: []string{"cli"}}
		results, stats := FilterRepositories(repos, filter)

		if stats.Passed != 2 {
			t.Errorf("Expected 2 passed, got %d", stats.Passed)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("date filter", func(t *testing.T) {
		filter := &RepoFilter{UpdatedWithin: 30 * 24 * time.Hour}
		results, stats := FilterRepositories(repos, filter)

		if stats.DateFiltered != 1 {
			t.Errorf("Expected 1 date filtered, got %d", stats.DateFiltered)
		}
		if stats.Passed != 4 {
			t.Errorf("Expected 4 passed, got %d", stats.Passed)
		}
		if len(results) != 4 {
			t.Errorf("Expected 4 results, got %d", len(results))
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		filter := &RepoFilter{
			Languages:     []string{"Go"},
			Topics:        []string{"cli"},
			UpdatedWithin: 30 * 24 * time.Hour,
		}
		results, stats := FilterRepositories(repos, filter)

		// Should match: active-go-repo and test-go-cli (both Go, have cli topic, recent)
		if stats.Passed != 2 {
			t.Errorf("Expected 2 passed, got %d", stats.Passed)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})
}

func TestFilterStatistics(t *testing.T) {
	now := time.Now()

	repos := []*github.Repository{
		createTestRepo("repo1", "Go", []string{}, false, false, now),
		createTestRepo("repo2", "Python", []string{}, false, false, now),
		createTestRepo("repo3", "Go", []string{}, true, false, now),
		createTestRepo("repo4", "Go", []string{}, false, true, now),
		createTestRepo("repo5", "Java", []string{}, false, false, now.Add(-100*24*time.Hour)),
	}

	filter := &RepoFilter{
		Languages:     []string{"Go"},
		UpdatedWithin: 30 * 24 * time.Hour,
		SkipForks:     true,
	}

	_, stats := FilterRepositories(repos, filter)

	// Verify all stats are tracked correctly
	if stats.Total != 5 {
		t.Errorf("Expected total 5, got %d", stats.Total)
	}
	if stats.Archived != 1 {
		t.Errorf("Expected 1 archived, got %d", stats.Archived)
	}
	if stats.Forks != 1 {
		t.Errorf("Expected 1 fork, got %d", stats.Forks)
	}
	if stats.LangFiltered != 2 {
		t.Errorf("Expected 2 language filtered (Python and Java, but Java also old), got %d", stats.LangFiltered)
	}
	// repo1: Go, recent, not fork -> passed
	if stats.Passed != 1 {
		t.Errorf("Expected 1 passed, got %d", stats.Passed)
	}
}

func TestEmptyRepositoryList(t *testing.T) {
	filter := &RepoFilter{Languages: []string{"Go"}}
	results, stats := FilterRepositories([]*github.Repository{}, filter)

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
	if stats.Total != 0 {
		t.Errorf("Expected total 0, got %d", stats.Total)
	}
	if stats.Passed != 0 {
		t.Errorf("Expected 0 passed, got %d", stats.Passed)
	}
}

func TestFilterWithNilRepository(t *testing.T) {
	// Ensure no panic with nil repository fields
	repos := []*github.Repository{
		{
			Name:     github.String("test"),
			FullName: github.String("owner/test"),
		},
	}

	filter := &RepoFilter{}
	results, stats := FilterRepositories(repos, filter)

	// Should handle gracefully
	if stats.Total != 1 {
		t.Errorf("Expected total 1, got %d", stats.Total)
	}
	if len(results) >= 0 {
		// Just checking it doesn't panic
		t.Logf("Got %d results", len(results))
	}
}

func TestComplexNamePatterns(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		pattern       string
		repoNames     []string
		expectedMatch int
	}{
		{
			name:          "prefix match",
			pattern:       "^test-",
			repoNames:     []string{"test-repo", "test-app", "my-test", "testing"},
			expectedMatch: 2,
		},
		{
			name:          "suffix match",
			pattern:       "-api$",
			repoNames:     []string{"user-api", "product-api", "api-server", "api"},
			expectedMatch: 2,
		},
		{
			name:          "contains match",
			pattern:       "service",
			repoNames:     []string{"user-service", "service-api", "backend", "microservice"},
			expectedMatch: 3,
		},
		{
			name:          "wildcard pattern",
			pattern:       "^app-.*-v[0-9]+$",
			repoNames:     []string{"app-web-v1", "app-api-v2", "app-web", "web-app-v1"},
			expectedMatch: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &RepoFilter{
				NamePattern: regexp.MustCompile(tt.pattern),
			}

			var repos []*github.Repository
			for _, name := range tt.repoNames {
				repos = append(repos, createTestRepo(name, "Go", []string{}, false, false, now))
			}

			results, _ := FilterRepositories(repos, filter)
			if len(results) != tt.expectedMatch {
				t.Errorf("Expected %d matches, got %d", tt.expectedMatch, len(results))
			}
		})
	}
}

func TestMultipleTopicsRequirement(t *testing.T) {
	now := time.Now()

	repos := []*github.Repository{
		createTestRepo("repo1", "Go", []string{"cli", "tool", "golang"}, false, false, now),
		createTestRepo("repo2", "Go", []string{"cli", "golang"}, false, false, now),
		createTestRepo("repo3", "Go", []string{"cli"}, false, false, now),
		createTestRepo("repo4", "Go", []string{"tool", "golang"}, false, false, now),
	}

	// Require both "cli" AND "golang"
	filter := &RepoFilter{
		Topics: []string{"cli", "golang"},
	}

	results, stats := FilterRepositories(repos, filter)

	// Only repo1 and repo2 have both "cli" and "golang"
	if stats.Passed != 2 {
		t.Errorf("Expected 2 passed (have both cli and golang), got %d", stats.Passed)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}
