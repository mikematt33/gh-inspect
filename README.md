# gh-inspect

**gh-inspect** is an opinionated, deep-inspection CLI tool designed to measure the engineering health of GitHub repositories. It goes beyond simple metrics, analyzing commit patterns, PR velocity, issue hygiene, and CI stability to provide a comprehensive "Health Score" for your project.

![Build Status](https://github.com/mikematt33/gh-inspect/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)

## 🚀 Key Features

- **Engineering Health Score**: Aggregates hundreds of data points into a single 0-100 score.
- **Bus Factor Analysis**: Identifies if your project relies too heavily on single contributors.
- **PR Velocity**: Measures Cycle Time, Reviews per PR, and identifies "Giant PRs" that slow down development.
- **Zombie Detection**: Finds stale issues and PRs that are clogging up your backlog.
- **CI Insights**: Tracks workflow success rates, expensive runs, and stability.
- **CI/CD Gates**: Use `--fail-under` to block merges if repository health drops below a certain threshold.

## 🛠️ Installation

### Quick Install (Recommended)

You can install the latest version directly from GitHub using curl:

```bash
curl -sfL https://raw.githubusercontent.com/mikematt33/gh-inspect/main/install.sh | sh
```

To install a specific version (e.g., v0.1.0):

```bash
curl -sfL https://raw.githubusercontent.com/mikematt33/gh-inspect/main/install.sh | sh -s -- -v v0.1.0
```

### Build from Source

Requirements: Go 1.24+

```bash
# Clone the repository
git clone https://github.com/mikematt33/gh-inspect.git
cd gh-inspect

# Build using Make
make build

# Verify installation
./bin/gh-inspect --help
```

## 🔧 System Requirements

- **Go Version**: 1.24.0 or higher
- **GitHub Token**: Required for accessing GitHub API (5000 requests/hour with authentication)
- **Rate Limit Considerations**: The tool includes smart API call optimization:
  - Repository data caching (reduces duplicate calls by 2-3 per repo)
  - Time-windowed queries (only fetches data within analysis period)
  - Intelligent pagination limits (up to 5000 workflow runs, 1000 issues, etc.)
  - Pre-flight rate limit checks with warnings

## ⚡ Quick Start

1.  **Authentication**:

    ```bash
    gh-inspect auth
    ```

    _This will help you login via GitHub CLI or paste a token._

2.  **Run Analysis**:
    Analyze a specific repository.
    _(The CLI will automatically create a configuration file on first run)_

    ```bash
    gh-inspect run owner/repo
    ```

    _Note: If you built from source, use `./bin/gh-inspect` instead of `gh-inspect`._

## ⚙️ Usage Details

### Available Commands

#### `auth` - Authenticate

Log in to GitHub to access private repos and increase rate limits.

```bash
# Show available auth commands
gh-inspect auth

# Log in interactively
gh-inspect auth login

# Check authentication status (shows rate limits with human-readable time)
gh-inspect auth status

# Log out (removes tokens from all locations: config, shell files, gh CLI)
gh-inspect auth logout

# For servers without a browser (uses device code flow)
gh-inspect auth login --no-browser
```

**Token Storage Options:**

When logging in, you can choose how to store your GitHub token:

1. **Temporary** (session only) - Export to current terminal
2. **Persistent shell** - Add to `.bashrc` or `.zshrc` for all sessions
3. **Config file** - Store in gh-inspect configuration (shown with security warning)
4. **Don't store** - Use token once, don't save

**Auth Status Features:**

The `auth status` command shows:

- Current authentication status
- Token source (config file, environment variable, or gh CLI)
- Rate limit remaining/total
- Reset time in both RFC3339 and human-readable format (e.g., "in 45 minutes")

**Logout Features:**

The `auth logout` command intelligently:

- Detects tokens in all locations (config, shell files, environment variables, gh CLI)
- Shows all found token locations
- Removes tokens from config file and shell rc files automatically
- Provides instructions for manual removal of environment variables and gh CLI tokens

#### `run` - Analyze Repositories

Analyze one or more repositories (format: owner/repo).

```bash
gh-inspect run owner/repo [flags]
```

**Flags:**

- `-d, --deep`: Enable deep scanning (paginated issues/PRs).
- `-f, --format string`: Output format (text, json) (default "text").
- `-s, --since string`: Lookback window (e.g. 30d, 24h) (default "30d").
- `--fail-under int`: Exit with error code 1 if average health score is below this value.
- `--include strings`: Only run specified analyzers (comma-separated: activity,prflow,ci,issues,security,releases,branches,health).
- `--exclude strings`: Exclude specified analyzers (comma-separated: activity,prflow,ci,issues,security,releases,branches,health).
- `--list-analyzers`: List all available analyzers with descriptions and exit.

**Global Flags:**

- `-q, --quiet`: Suppress non-essential output (useful for CI/CD).
- `-v, --verbose`: Enable verbose output with detailed progress information.

**Progress Indicator:**

During analysis, a clean progress bar shows:

- Current progress: `Analyzing repositories (5/10)`
- Automatically clears when complete for clean output
- Can be suppressed with `--quiet` flag for CI/CD pipelines

#### `org` - Organization Scan

Scan all active repositories in a GitHub organization. Automatically skips archived repositories.

```bash
gh-inspect org organization [flags]
```

**Features:**

- Analyzes all non-archived repositories in the organization
- Provides aggregated organization-level summary including:
  - Total repositories analyzed
  - Average health score across all repos
  - Average PR cycle time
  - Average CI success rate
  - **Average CI runtime** 🆕 - Mean build time across all repos
  - Total commits, issues, and findings
  - Repos at risk (health score < 50)
  - Repos with bus factor of 1

**Flags:**

- Uses the same flags as `run` (`--deep`, `--format`, `--since`, `--fail-under`, `--include`, `--exclude`).

#### `user` - User Scan

Analyze all repositories belonging to a specific user.

```bash
gh-inspect user username [flags]
```

**Features:**

- Analyzes all repositories owned by the user
- Gracefully handles empty repositories (shows info message instead of error)
- Provides same aggregated summary as organization scans

**Flags:**

- Uses the same flags as `run` (`--deep`, `--format`, `--since`, `--fail-under`, `--include`, `--exclude`).

#### `compare` - Compare Repositories

Compare metrics of multiple repositories side-by-side. Useful for benchmarking.

```bash
gh-inspect compare owner/repo1 owner/repo2 [flags]
```

**Flags:**

- `-d, --deep`: Enable deep scanning.
- `-f, --format string`: Output format (text, json).
- `-s, --since string`: Lookback window.

#### `update`

Update `gh-inspect` to the latest version.

```bash
# Update to the latest version
gh-inspect update

# Check for updates without installing
gh-inspect update --check
```

#### `uninstall`

Uninstall the CLI from your system.

```bash
gh-inspect uninstall
```

#### `completion`

Generate and manage shell completion scripts for bash, zsh, fish, and PowerShell.

**Automatic Setup:**

```bash
gh-inspect completion --auto
```

This will detect your shell and configure completions automatically.

**Check Completion Status:**

```bash
gh-inspect completion status
```

Verifies if your installed completions match the current version and warns if they're outdated.

**Smart Completions:**

Completions support:

- ✅ **All flags and commands** - Auto-generated from Cobra
- ✅ **Recent repositories** - Suggests previously analyzed repos
- ✅ **Organizations** - Lists your GitHub organizations
- ✅ **Users** - Includes your authenticated user and recent users
- ✅ **Auto-update detection** - Warns when completions are stale
- ✅ **Smart replacement** - `--auto` flag replaces outdated completions instead of duplicating them

**Manual Setup:**

Run `gh-inspect completion <shell> --help` for shell-specific instructions.

```bash
# Bash
source <(gh-inspect completion bash)

# Zsh
source <(gh-inspect completion zsh)

# Fish
gh-inspect completion fish | source
```

**Regenerate After Updates:**

When you update gh-inspect to a new version with new commands, regenerate completions:

```bash
gh-inspect completion --auto
```

#### `init` & `config`

Initialize or manage configuration. See [Configuration](#-configuration) for details.

### Examples

**Deep Scan (Last 90 days)**
Performs a more intensive scan, including issue pagination and deep metrics.

```bash
gh-inspect run owner/repo --deep --since=90d
```

**JSON Output**
Useful for piping into other tools `jq`.

```bash
gh-inspect run owner/repo --format=json > report.json
```

**Quality Gate**
Fail the command (exit code 1) if the health score is below 80. Perfect for CI pipelines.

```bash
gh-inspect run owner/repo --fail-under=80
```

**Quiet Mode for CI/CD**
Suppress progress output for cleaner CI logs.

```bash
gh-inspect run owner/repo --quiet --format=json > report.json
```

**List Available Analyzers**
See all available analyzers with their descriptions.

```bash
gh-inspect run --list-analyzers
# or with any command:
gh-inspect org --list-analyzers
gh-inspect user --list-analyzers
gh-inspect compare --list-analyzers
```

**Selective Analyzers with Include**
Run only specific analyzers when you need targeted analysis.

```bash
# Only check activity, CI status, and security
gh-inspect run owner/repo --include=activity,ci,security
```

**Exclude Analyzers**
Skip analyzers you don't need to save API rate limits and time.

```bash
# Skip releases and branches analysis
gh-inspect run owner/repo --exclude=releases,branches
```

**Available Analyzers:**

- `activity` - Commit patterns, contributors, bus factor
- `prflow` - Pull request velocity and cycle time
- `ci` - CI/CD workflow success rates
- `issues` - Issue hygiene and zombie detection
- `security` - Security advisories and vulnerabilities
- `releases` - Release frequency and patterns
- `branches` - Branch protection and stale branches
- `health` - Repository health files (README, LICENSE, etc.)

**Verbose Mode**
Get detailed progress information during long-running analyses.

```bash
gh-inspect run owner/org --verbose
```

## ⚙️ Configuration

Run `init` to generate a default configuration if one does not exist (this happens automatically on first run):

```bash
gh-inspect init
```

### Configuration File Location

The configuration file is stored in your user configuration directory:

- **Linux:** `~/.config/gh-inspect/config.yaml`
- **macOS:** `~/Library/Application Support/gh-inspect/config.yaml`
- **Windows:** `%APPDATA%\gh-inspect\config.yaml`

### Managing Configuration via CLI

You can view and modify configuration values directly from the CLI without editing the file manually.

**List current configuration:**

```bash
gh-inspect config list
```

**Set a value:**
Use dot notation to target specific fields (snake_case keys).

```bash
# Set your GitHub token
gh-inspect config set-token ghp_123456...

# Change the zombie issue threshold to 90 days
gh-inspect config set analyzers.issue_hygiene.params.zombie_threshold_days 90

# Set stale branch threshold to 60 days
gh-inspect config set analyzers.branches.params.stale_threshold_days 60

# Disable specific analyzers
gh-inspect config set analyzers.ci.enabled false
gh-inspect config set analyzers.security.enabled false

# Set specific concurrency limit
gh-inspect config set global.concurrency 10
```

### Configurable Analyzers

All analyzers can be enabled/disabled and configured:

- **activity** - Always enabled (core metrics)
- **pr_flow** - Enabled by default, configurable stale threshold
- **issue_hygiene** - Enabled by default, configurable stale/zombie thresholds
- **repo_health** - Enabled by default
- **ci** - Enabled by default
- **security** 🆕 - Enabled by default (gracefully handles missing GHAS)
- **releases** 🆕 - Enabled by default
- **branches** 🆕 - Enabled by default, configurable stale threshold (90 days)

## 🔍 Included Analyzers

gh-inspect includes 7 comprehensive analyzers that examine different aspects of your repository health:

| Analyzer          | Description                      | Key Metrics                                                         |
| ----------------- | -------------------------------- | ------------------------------------------------------------------- |
| **Activity**      | Contributor engagement & growth  | Bus Factor, Stars/Forks, New Contributors, Commit Velocity          |
| **PR Flow**       | Review velocity & quality        | Cycle Time, Self-Merge Rate, Draft Adoption, Description Quality    |
| **Issue Hygiene** | Backlog health & responsiveness  | Time to First Response, Assignee Coverage, Bug/Feature Ratio        |
| **Repo Health**   | Governance & best practices      | Branch Protection, Dependency Management, Key Files (LICENSE, etc.) |
| **CI Stability**  | Build health & reliability       | Success Rate, Workflow Cost, Average Runtime                        |
| **Security** 🆕   | Vulnerability & secret detection | Dependabot Alerts, Secret Scanning, Code Scanning                   |
| **Releases** 🆕   | Release management & cadence     | Release Frequency, Changelog Coverage, Semantic Versioning          |
| **Branches** 🆕   | Branch management                | Total Branches, Stale Branches, Branch Health                       |

### Analyzer Details

#### Activity Analyzer

Tracks contributor engagement and repository popularity:

- **Commits Total** - Number of commits in the analysis window
- **Commit Velocity** - Average commits per day
- **Bus Factor** - Number of authors accounting for 50% of commits
- **Active Contributors** - Total distinct commit authors
- **New Contributors** 🆕 - First-time contributors in the window
- **Stars** 🆕 - Repository star count
- **Forks** 🆕 - Repository fork count
- **Watchers** 🆕 - Repository watchers count

#### PR Flow Analyzer

Analyzes pull request efficiency and quality:

- **Avg Cycle Time** - Time from PR creation to merge
- **Avg Time to First Review** 🆕 - How quickly PRs get initial feedback
- **Avg Approvals per PR** 🆕 - Review engagement level
- **Merge Ratio** - Percentage of PRs that get merged
- **Self-Merge Rate** 🆕 - PRs merged by their own author
- **Draft PR Rate** 🆕 - Adoption of draft PR workflow
- **Description Quality** 🆕 - PRs with meaningful descriptions
- **Avg PR Size** - Lines changed per PR

#### Issue Hygiene Analyzer

Measures issue management effectiveness:

- **Open Issues Total** - Current open issue count
- **Closed Issues in Window** - Issues resolved in the period
- **Avg Issue Lifetime** - Time to close issues
- **Avg First Response Time** 🆕 - Speed of initial triage
- **Label Coverage** - Issues properly tagged
- **Assignee Coverage** 🆕 - Issues with assigned owners
- **Issue-PR Link Rate** 🆕 - Issues linked to PRs
- **Bug Count** 🆕 - Open bug issues
- **Feature Count** 🆕 - Open feature requests
- **Stale Issues** - Inactive beyond threshold
- **Zombie Issues** - Very old open issues

#### Repo Health Analyzer

Evaluates repository governance and standards:

- **Health Score** - Composite score (0-100)
- **Key Files Present** - LICENSE, README, CONTRIBUTING, SECURITY, CODE_OF_CONDUCT 🆕, CODEOWNERS
- **CI Status** - Status of default branch
- **Branch Protection** 🆕 - Protection rules configured
- **Requires PR Reviews** 🆕 - Review requirement setting
- **Requires Status Checks** 🆕 - CI requirement setting
- **Dependency Management** 🆕 - Package manager detected
- **Default Branch** 🆕 - Primary branch name

#### CI Stability Analyzer

Monitors continuous integration health:

- **Workflow Runs All Time** 🆕 - Total CI executions ever (accurate count from API)
- **Workflow Runs in Window** 🆕 - CI executions in analysis period
- **Workflow Runs Analyzed** 🆕 - Sample size used for statistics (up to 5000)
- **Unique Workflows** 🆕 - Number of different workflow files
- **Success Count** 🆕 - Number of successful runs
- **Failure Count** 🆕 - Number of failed runs
- **Cancelled Count** 🆕 - Number of cancelled runs
- **Success Rate** - Percentage of passing runs
- **Avg Runtime** - Mean workflow duration
- Identifies expensive workflows and flaky tests

**Note:** Statistics (success rate, avg runtime) are calculated from up to 5000 recent runs within the analysis window, providing highly accurate insights without exhausting API rate limits.

#### Security Analyzer 🆕

Scans for vulnerabilities and security issues:

- **Dependabot Alerts** - Total open alerts by severity (Critical, High, Medium, Low)
- **Secret Scanning Alerts** - Potential leaked credentials
- **Code Scanning Alerts** - Static analysis findings
- Requires GitHub Advanced Security for private repos

#### Releases Analyzer 🆕

Tracks release management practices:

- **Releases in Window** - Number of releases created
- **Release Frequency** - Average releases per month
- **Avg Days Between Releases** - Release cadence
- **Changelog Coverage** - Releases with notes
- **Semver Compliance** - Semantic versioning adoption
- **Pre-release Ratio** - Beta vs stable releases

#### Branches Analyzer 🆕

Monitors branch management hygiene:

- **Total Branches** - All branches in repository
- **Stale Branches** - Branches inactive beyond threshold (default: 90 days)
- Flags repositories with too many branches (>50)
- Identifies cleanup opportunities

All analyzers work with `run`, `org`, `user`, and `compare` commands!

## ⚡ Performance & API Optimization

gh-inspect is designed to provide comprehensive analysis while minimizing API calls and respecting GitHub's rate limits.

### Smart API Call Management

**Repository Data Caching:**

- Repository metadata is cached in-memory per session
- Reduces duplicate API calls when multiple analyzers need the same data
- Saves 2-3 API calls per repository analyzed

**Time-Windowed Queries:**

- Only fetches data within the specified analysis period (default: 30 days)
- Uses GitHub's built-in filtering to avoid retrieving unnecessary historical data
- Significantly reduces pagination for active repositories

**Intelligent Pagination:**

- Workflow runs: Up to 5,000 analyzed (with accurate all-time total from API)
- Issues: Automatically paginated with 100 per page
- Pull requests: Smart sampling for large repositories
- Commits: Time-bounded to analysis window

**Rate Limit Protection:**

- Pre-flight checks estimate API cost before analysis
- Warns if rate limit might be exhausted
- Automatic rate limit monitoring with sleep/retry on exhaustion
- Real-time rate limit display in `auth status` command

### Typical API Cost

For a **moderately active repository** (50-100 commits/week) with default 30d window:

- **Minimal scan**: ~15-25 API calls per repository
- **Deep scan**: ~30-50 API calls per repository

With authentication, you have 5,000 requests/hour, which allows analyzing:

- **200-300 repositories** in a single organization scan
- **Multiple teams** across different analysis windows

### Empty Repository Handling

Empty repositories are gracefully handled:

- No error thrown for repositories with no commits
- Shows informational finding: "Repository is empty (no commits)"
- Continues analyzing other repositories normally

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to set up your development environment, run tests, and open Pull Requests.

```bash
# Run tests locally
make test

# Run linters
make lint
```

## 📄 License

This project is licensed under the terms of the LICENSE file included in this repository.
