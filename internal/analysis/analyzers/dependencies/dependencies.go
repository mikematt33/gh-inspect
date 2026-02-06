package dependencies

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mikematt33/gh-inspect/internal/analysis"
	"github.com/mikematt33/gh-inspect/pkg/models"
	"gopkg.in/yaml.v3"
)

type Analyzer struct{}

func New() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Name() string {
	return "dependencies"
}

// PackageManager represents different dependency management systems
type PackageManager struct {
	Name     string
	Files    []string
	Language string
}

var packageManagers = []PackageManager{
	{Name: "npm", Files: []string{"package.json", "package-lock.json"}, Language: "JavaScript"},
	{Name: "yarn", Files: []string{"yarn.lock"}, Language: "JavaScript"},
	{Name: "pnpm", Files: []string{"pnpm-lock.yaml"}, Language: "JavaScript"},
	{Name: "go-modules", Files: []string{"go.mod", "go.sum"}, Language: "Go"},
	{Name: "pip", Files: []string{"requirements.txt", "setup.py", "pyproject.toml"}, Language: "Python"},
	{Name: "pipenv", Files: []string{"Pipfile", "Pipfile.lock"}, Language: "Python"},
	{Name: "poetry", Files: []string{"poetry.lock"}, Language: "Python"},
	{Name: "cargo", Files: []string{"Cargo.toml", "Cargo.lock"}, Language: "Rust"},
	{Name: "maven", Files: []string{"pom.xml"}, Language: "Java"},
	{Name: "gradle", Files: []string{"build.gradle", "build.gradle.kts"}, Language: "Java"},
	{Name: "bundler", Files: []string{"Gemfile", "Gemfile.lock"}, Language: "Ruby"},
	{Name: "composer", Files: []string{"composer.json", "composer.lock"}, Language: "PHP"},
	{Name: "nuget", Files: []string{"packages.config", ".csproj"}, Language: "C#"},
}

func (a *Analyzer) Analyze(ctx context.Context, client analysis.Client, repo analysis.TargetRepository, cfg analysis.Config) (models.AnalyzerResult, error) {
	var metrics []models.Metric
	var findings []models.Finding

	// Detect package managers by checking for their files
	detectedManagers := make(map[string]bool)
	dependencyFiles := make(map[string]string) // filename -> content

	for _, pm := range packageManagers {
		for _, file := range pm.Files {
			fileContent, _, err := client.GetContent(ctx, repo.Owner, repo.Name, file)
			if err == nil && fileContent != nil && fileContent.Content != nil {
				content, err := fileContent.GetContent()
				if err == nil && content != "" {
					detectedManagers[pm.Name] = true
					dependencyFiles[file] = content
					break // Found one file for this manager
				}
			}
		}
	}

	if len(detectedManagers) == 0 {
		metrics = append(metrics, models.Metric{
			Key:          "package_managers",
			Value:        0,
			DisplayValue: "0",
			Description:  "No dependency management detected",
		})

		findings = append(findings, models.Finding{
			Type:        "no_dependency_management",
			Severity:    models.SeverityInfo,
			Message:     "No package manager detected",
			Explanation: "Using a package manager helps track dependencies and ensures reproducible builds.",
			SuggestedActions: []string{
				"Consider adding a package manager for your language",
				"Document dependencies in a standard manifest file",
			},
		})

		return models.AnalyzerResult{
			Name:     a.Name(),
			Metrics:  metrics,
			Findings: findings,
		}, nil
	}

	// Report detected package managers
	var pmList []string
	for pm := range detectedManagers {
		pmList = append(pmList, pm)
	}

	metrics = append(metrics, models.Metric{
		Key:          "package_managers",
		Value:        float64(len(detectedManagers)),
		Unit:         "count",
		DisplayValue: strings.Join(pmList, ", "),
		Description:  "Detected package managers",
	})

	// Analyze specific dependency files
	totalDeps := 0

	// Parse package.json if available
	if content, exists := dependencyFiles["package.json"]; exists {
		deps, devCount := parsePackageJSON(content)
		totalDeps += deps

		metrics = append(metrics, models.Metric{
			Key:          "npm_dependencies",
			Value:        float64(deps),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", deps),
			Description:  "NPM dependencies",
		})

		if devCount > 0 {
			metrics = append(metrics, models.Metric{
				Key:          "npm_dev_dependencies",
				Value:        float64(devCount),
				Unit:         "count",
				DisplayValue: fmt.Sprintf("%d", devCount),
				Description:  "NPM dev dependencies",
			})
		}
	}

	// Parse go.mod if available
	if content, exists := dependencyFiles["go.mod"]; exists {
		deps := parseGoMod(content)
		totalDeps += deps

		metrics = append(metrics, models.Metric{
			Key:          "go_dependencies",
			Value:        float64(deps),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", deps),
			Description:  "Go module dependencies",
		})
	}

	// Parse requirements.txt if available
	if content, exists := dependencyFiles["requirements.txt"]; exists {
		deps, pinnedCount := parseRequirementsTxt(content)
		totalDeps += deps

		metrics = append(metrics, models.Metric{
			Key:          "python_dependencies",
			Value:        float64(deps),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", deps),
			Description:  "Python dependencies",
		})

		if deps > 0 {
			pinnedRatio := float64(pinnedCount) / float64(deps) * 100
			metrics = append(metrics, models.Metric{
				Key:          "python_pinned_versions",
				Value:        pinnedRatio,
				Unit:         "percent",
				DisplayValue: fmt.Sprintf("%.0f%%", pinnedRatio),
				Description:  "Python dependencies with pinned versions",
			})

			if pinnedRatio < 50 {
				findings = append(findings, models.Finding{
					Type:        "unpinned_dependencies",
					Severity:    models.SeverityMedium,
					Message:     "Many Python dependencies lack version pins",
					Explanation: "Unpinned dependencies can lead to unexpected behavior when new versions are released.",
					SuggestedActions: []string{
						"Pin dependency versions using == operator",
						"Use pip freeze to generate pinned versions",
					},
				})
			}
		}
	}

	// Parse Cargo.toml if available
	if content, exists := dependencyFiles["Cargo.toml"]; exists {
		deps := parseCargoToml(content)
		totalDeps += deps

		metrics = append(metrics, models.Metric{
			Key:          "rust_dependencies",
			Value:        float64(deps),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", deps),
			Description:  "Rust dependencies",
		})
	}

	// Total dependencies metric
	if totalDeps > 0 {
		metrics = append(metrics, models.Metric{
			Key:          "total_dependencies",
			Value:        float64(totalDeps),
			Unit:         "count",
			DisplayValue: fmt.Sprintf("%d", totalDeps),
			Description:  "Total dependencies across all managers",
		})

		// Check for dependency bloat
		if totalDeps > 100 {
			findings = append(findings, models.Finding{
				Type:        "dependency_bloat",
				Severity:    models.SeverityLow,
				Message:     fmt.Sprintf("High dependency count (%d dependencies)", totalDeps),
				Explanation: "Many dependencies increase maintenance burden, security risks, and build times.",
				SuggestedActions: []string{
					"Audit dependencies and remove unused ones",
					"Consider consolidating similar dependencies",
				},
			})
		}
	}

	// Check for lock files (indicates version pinning)
	hasLockFile := false
	lockFiles := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum", "Pipfile.lock", "poetry.lock", "Cargo.lock", "Gemfile.lock", "composer.lock"}
	var foundLockFiles []string
	for _, lockFile := range lockFiles {
		if _, exists := dependencyFiles[lockFile]; exists {
			hasLockFile = true
			foundLockFiles = append(foundLockFiles, lockFile)
		}
	}

	if hasLockFile {
		metrics = append(metrics, models.Metric{
			Key:          "lock_files",
			Value:        float64(len(foundLockFiles)),
			Unit:         "count",
			DisplayValue: strings.Join(foundLockFiles, ", "),
			Description:  "Lock files present (ensures reproducible builds)",
		})
	} else if len(detectedManagers) > 0 {
		findings = append(findings, models.Finding{
			Type:        "missing_lock_file",
			Severity:    models.SeverityMedium,
			Message:     "No lock file detected",
			Explanation: "Lock files ensure reproducible builds by pinning exact dependency versions.",
			SuggestedActions: []string{
				"Commit lock files (package-lock.json, yarn.lock, etc.) to version control",
				"Enable lock file generation in your package manager",
			},
		})
	}

	return models.AnalyzerResult{
		Name:     a.Name(),
		Metrics:  metrics,
		Findings: findings,
	}, nil
}

// parsePackageJSON extracts dependency counts from package.json
func parsePackageJSON(content string) (int, int) {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return 0, 0
	}

	return len(pkg.Dependencies), len(pkg.DevDependencies)
}

// parseGoMod counts dependencies in go.mod
func parseGoMod(content string) int {
	lines := strings.Split(content, "\n")
	count := 0
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			if line != "" && !strings.HasPrefix(line, "//") {
				count++
			}
			continue
		}
		if strings.HasPrefix(line, "require ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "require"))
			if rest != "" && !strings.HasPrefix(rest, "//") {
				count++
			}
		}
	}

	return count
}

// parseRequirementsTxt counts dependencies and pinned versions
func parseRequirementsTxt(content string) (int, int) {
	lines := strings.Split(content, "\n")
	total := 0
	pinned := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		total++
		// Check if version is pinned with ==
		if strings.Contains(line, "==") {
			pinned++
		}
	}

	return total, pinned
}

// parseCargoToml counts dependencies in Cargo.toml
func parseCargoToml(content string) int {
	var cargo struct {
		Dependencies    map[string]interface{} `yaml:"dependencies"`
		DevDependencies map[string]interface{} `yaml:"dev-dependencies"`
	}

	if err := yaml.Unmarshal([]byte(content), &cargo); err != nil {
		return 0
	}

	return len(cargo.Dependencies) + len(cargo.DevDependencies)
}
