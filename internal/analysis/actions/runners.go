package actions

import (
	"strings"
)

// knownGitHubHostedPrefixes are the standard GitHub-hosted runner label roots.
var knownGitHubHostedPrefixes = []string{
	"ubuntu", "windows", "macos", "macos-latest", "ubuntu-latest", "windows-latest",
}

// deprecatedRunnerLabels are runner labels GitHub has deprecated or that are
// known to be retired/brownout targets.
var deprecatedRunnerLabels = map[string]bool{
	"ubuntu-18.04": true,
	"ubuntu-16.04": true,
	"macos-10.15":  true,
	"macos-11":     true,
	"windows-2016": true,
	"windows-2019": true, // retiring
}

// runnerClass describes how a job's runner labels map to hosting and OS.
type runnerClass struct {
	hosted     bool
	os         string // linux, windows, macos, unknown
	deprecated bool
	primary    string // the label used for attribution
}

// classifyRunner inspects a job's `runs-on` labels and determines whether the
// runner is GitHub-hosted or self-hosted, its OS, and whether it uses a
// deprecated label. The "self-hosted" label is authoritative for self-hosted.
func classifyRunner(labels []string) runnerClass {
	rc := runnerClass{os: "unknown"}
	if len(labels) == 0 {
		return rc
	}

	lower := make([]string, len(labels))
	for i, l := range labels {
		lower[i] = strings.ToLower(strings.TrimSpace(l))
	}
	rc.primary = lower[0]

	for _, l := range lower {
		if l == "self-hosted" {
			rc.hosted = false
			rc.os = osFromLabels(lower)
			rc.deprecated = anyDeprecated(lower)
			return rc
		}
	}

	// No explicit self-hosted label: treat standard labels as GitHub-hosted,
	// otherwise assume self-hosted (custom label).
	rc.os = osFromLabels(lower)
	rc.deprecated = anyDeprecated(lower)
	rc.hosted = isGitHubHosted(lower)
	return rc
}

func isGitHubHosted(lower []string) bool {
	for _, l := range lower {
		for _, p := range knownGitHubHostedPrefixes {
			if l == p || strings.HasPrefix(l, p+"-") {
				return true
			}
		}
	}
	return false
}

func osFromLabels(lower []string) string {
	for _, l := range lower {
		switch {
		case strings.Contains(l, "ubuntu") || strings.Contains(l, "linux"):
			return "linux"
		case strings.Contains(l, "windows"):
			return "windows"
		case strings.Contains(l, "macos") || strings.Contains(l, "mac"):
			return "macos"
		}
	}
	return "unknown"
}

func anyDeprecated(lower []string) bool {
	for _, l := range lower {
		if deprecatedRunnerLabels[l] {
			return true
		}
	}
	return false
}
