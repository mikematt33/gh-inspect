<img src="logo.svg" alt="gh-inspect logo" width="500">

# gh-inspect

**gh-inspect** is an opinionated, deep-inspection CLI that measures the engineering health of GitHub repositories. It analyzes commit patterns, PR velocity, issue hygiene, CI stability, security posture, and more, distilling hundreds of data points into a single 0–100 **Health Score**. It also ships a dedicated `actions` command for deep GitHub Actions analytics — workflow run health, duration percentiles, runner usage, and org-wide roll-ups — backed by quota-aware multi-token and GitHub App authentication.

Run it against a single repo, an entire organization, or every repo a user owns; output as text, JSON, or Markdown for dashboards, CI gates, and PR comments.

![Build Status](https://github.com/mikematt33/gh-inspect/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8.svg)

## 📑 Table of Contents

- [Key Features](#-key-features)
- [Installation](#️-installation)
- [System Requirements](#-system-requirements)
- [Quick Start](#-quick-start)
- [Usage Details](#️-usage-details)
  - [Available Commands](#available-commands)
  - [`actions` — GitHub Actions Analytics](#actions---github-actions-analytics)
  - [GitHub Actions Integration](#github-actions-integration)
- [Enterprise Scaling & Rate Limiting](#-enterprise-scaling--rate-limiting)
- [Configuration](#️-configuration)
- [Included Analyzers](#-included-analyzers)
- [Recommendations Engine](#-recommendations-engine)
- [Performance & API Optimization](#-performance--api-optimization)
- [Contributing](#-contributing)
- [License](#-license)

## 🚀 Key Features

**Scoring & insights**

- **Engineering Health Score** — aggregates hundreds of data points into a single 0–100 score.
- **Score Explanation** — `--explain` shows exactly why your score changed, with improvement tips.
- **Recommendations Engine** — actionable suggestions with "why this matters" context for every finding.
- **Flexible Output Modes** — suggestive (advice), observational (neutral facts), or statistical (numbers only).

**Tracking over time**

- **Baseline & Regression Detection** — snapshot scores and fail CI automatically when they drop.
- **Baseline History & Trends** — compare the current run against recent timestamped snapshots.
- **Remediation Tracking** — stable finding IDs tracked as open, in-progress, resolved, accepted, or ignored.

**What it analyzes**

- **Activity & Bus Factor** — commit velocity, contributor growth, and single-maintainer risk.
- **PR Velocity & Collaboration** — cycle time, review coverage, reviewer engagement, and "Giant PR" detection.
- **Issue Hygiene** — stale/zombie detection, response times, and backlog health.
- **CI & GitHub Actions Analytics** — workflow success rates, expensive runs, plus a dedicated `actions` command for run health, duration percentiles, runner usage, queue time, scheduled-workflow monitoring, and org roll-ups.
- **Deployment & Dependencies** — release cadence, changelog coverage, and multi-language dependency insights.

**Scale & automation**

- **Single repo, org, or user** scans with **repository filtering** by name, language, topics, and update date.
- **Smart Depth Control** — shallow, standard, or deep analysis with fine-grained API budgeting.
- **Quota-aware auth** — multi-token rotation and GitHub App installation tokens with pre-flight cost estimation.
- **CI/CD Gates** — `--fail-under` and `--fail-on-regression` to block merges on poor health.
- **Markdown output** optimized for PR comments and GitHub Actions step summaries.

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

#### `compare` - Compare Repositories

Compare metrics of multiple repositories side-by-side. Useful for benchmarking.

```bash
gh-inspect compare owner/repo1 owner/repo2 [flags]
```

**Flags:**

- `--depth string`: Analysis depth.
- `--max-prs int`, `--max-issues int`, `--max-workflow-runs int`: Resource limits.
- `-f, --format string`: Output format (text, json, markdown).
- `-s, --since string`: Lookback window.
- `--explain`: Show score breakdown.
- `--baseline string`, `--save-baseline`, `--compare-last`, `--fail-on-regression`: Baseline comparison.
- `--list-analyzers`: List available analyzers.

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

- Uses the same flags as `run` (`--depth`, `--max-prs`, `--max-issues`, `--max-workflow-runs`, `--format`, `--since`, `--explain`, `--baseline`, `--save-baseline`, `--compare-last`, `--compare-history`, `--fail-on-regression`, `--fail-under`, `--include`, `--exclude`, `--remediation-file`).
- **Repository Filtering:** `--filter-name`, `--filter-language`, `--filter-topics`, `--filter-updated`, `--filter-skip-forks`

**Filtering Examples:**

```bash
# Only analyze Go and Python repositories
gh-inspect org my-org --filter-language=go,python

# Filter by name pattern (regex)
gh-inspect org my-org --filter-name="^api-.*"

# Only production repositories updated in last 90 days
gh-inspect org my-org --filter-topics=production --filter-updated=90d

# Skip forked repositories
gh-inspect org my-org --filter-skip-forks
```

#### `actions` - GitHub Actions Analytics

Deep analytics on GitHub Actions across one or more repositories or an entire organization.

```bash
gh-inspect actions [owner/repo...] [flags]
```

**What it reports:**

- **Workflow inventory** — name, path, trigger events, and enabled/disabled state
- **Run health** — total/success/failure/cancelled/skipped counts, success rate, MTBF, and flakiness detection
- **Duration metrics** — average, median, p95, max run duration, plus a slow-down trend
- **Runner usage** — GitHub-hosted vs. self-hosted breakdown by OS, with deprecated-label warnings
- **Queue time** — time between a run being queued and starting (when available)
- **Job-level breakdown** — slowest jobs bottlenecking a workflow (with `--sample-jobs`)
- **Scheduled workflows** — last run, last conclusion, and next expected run (parsed from `cron`)
- **Org roll-up** — per-repo summaries plus compute minutes (hosted vs. self-hosted) and failure hotspots

**Flags:**

- `--repo owner/repo` (repeatable), `--org <organization>`
- `--days <n>` lookback window, `--max-runs <n>` cap per repo, `--workflow <file|name>` to scope to one workflow
- `--top <n>` slowest jobs / hotspots, `--sample-jobs <n>` recent runs per workflow to inspect for job/runner detail
- `--format text|json|markdown`, `--output-file <path>`
- `--dry-run` (pre-flight estimate only), `--confirm` / `--yes` / `-y` (proceed past pre-flight without prompting; fails if quota is insufficient)
- **Auth:** `--token` (repeatable PATs), `--app-id` / `--app-key` / `--installation-id` (GitHub App)

**Examples:**

```bash
gh-inspect actions --repo owner/repo
gh-inspect actions --repo owner/repo1 --repo owner/repo2
gh-inspect actions --org myorg
gh-inspect actions --repo owner/repo --days 60
gh-inspect actions --repo owner/repo --workflow ci.yml
gh-inspect actions --org myorg --top 10
gh-inspect actions --org myorg --dry-run
gh-inspect actions --org myorg --confirm
gh-inspect actions --token ghp_aaa --token ghp_bbb --repo owner/repo
gh-inspect actions --app-id 12345 --app-key ./private-key.pem --installation-id 67890 --org myorg
```

**Authentication & token management:**

- **Multi-PAT rotation** — provide multiple PATs via `--token` (repeatable), the `GITHUB_TOKENS` env var, or `global.github_tokens` in config. The pool tracks each token's remaining quota independently and automatically routes each request to the token with the most remaining quota, rotating away on `403`/`429`. Exhausted or invalid tokens are surfaced at the end of a run.
- **GitHub App** — supply an App ID, installation ID, and private key (PEM file or inline) via flags or `global.github_apps` in config. Short-lived installation tokens are generated on demand and auto-refreshed before their one-hour expiry. Multiple installations are supported.
- **Pre-flight cost estimation** — before a large scan the tool estimates the repos in scope, workflow count, and total API calls required, then compares that against the available quota across all configured credentials. Use `--dry-run` to estimate without scanning, and `--confirm` to proceed non-interactively (the scan fails fast if quota is insufficient). The confirmation threshold is configurable via `actions.confirm_threshold`.

**Required permissions:**

- **PAT (classic):** `repo` (private) or `public_repo`, plus `read:org` for org scans. The `workflow` scope is _not_ required for read-only analytics.
- **PAT (fine-grained):** `Actions: read`, `Metadata: read`, and `Administration: read` (the latter only for self-hosted runner detail).
- **GitHub App:** `actions:read`, `metadata:read`, and `administration:read`.

#### `run` - Analyze Repositories

Analyze one or more repositories (format: owner/repo).

```bash
gh-inspect run owner/repo [flags]
```

**Flags:**

- `--depth string`: Analysis depth: shallow, standard, or deep (default "standard").
- `--max-prs int`: Maximum PRs to analyze (0 = use depth default).
- `--max-issues int`: Maximum issues to fetch (0 = use depth default).
- `--max-workflow-runs int`: Maximum CI runs to analyze (0 = use depth default).
- `-f, --format string`: Output format (text, json, markdown) (default "text").
- `-s, --since string`: Lookback window (e.g. 30d, 24h) (default "30d").
- `--explain`: Show detailed score breakdown and improvement tips.
- `--output-mode string`: Control how findings are presented: suggestive, observational, or statistical (default "observational").
- `--baseline string`: Path to baseline file to compare against.
- `--save-baseline`: Save this run as the new baseline.
- `--compare-last`: Compare with last saved baseline.
- `--fail-on-regression`: Exit with error if regression detected.
- `--compare-history int`: Compare against the last N baseline snapshots and print a trend summary.
- `--fail-under int`: Exit with error code 1 if average health score is below this value.
- `--include strings`: Only run specified analyzers (comma-separated: activity,prflow,ci,issues,security,releases,branches,health,dependencies).
- `--exclude strings`: Exclude specified analyzers (comma-separated: activity,prflow,ci,issues,security,releases,branches,health,dependencies).
- `--list-analyzers`: List all available analyzers with descriptions and exit.
- `--remediation-file string`: Path to remediation tracking file.

**Global Flags:**

- `-q, --quiet`: Suppress non-essential output (useful for CI/CD).
- `-v, --verbose`: Enable verbose output with detailed progress information.

**Progress Indicator:**

During analysis, a clean progress bar shows:

- Current progress: `Analyzing repositories (5/10)`
- Automatically clears when complete for clean output
- Can be suppressed with `--quiet` flag for CI/CD pipelines

#### `remediation`

Track and update finding remediation state.

```bash
gh-inspect remediation list owner/repo
gh-inspect remediation set-status <finding-id> in-progress --note="Assigned to platform team"
```

Supported remediation states:

- `open`
- `in-progress`
- `resolved`
- `accepted`
- `ignored`

#### `uninstall`

Uninstall the CLI from your system.

```bash
gh-inspect uninstall
```

#### `update`

Update `gh-inspect` to the latest version.

```bash
# Update to the latest version
gh-inspect update

# Check for updates without installing
gh-inspect update --check
```

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

- Uses the same flags as `run` (`--depth`, `--max-prs`, `--max-issues`, `--max-workflow-runs`, `--format`, `--since`, `--explain`, `--baseline`, `--save-baseline`, `--compare-last`, `--compare-history`, `--fail-on-regression`, `--fail-under`, `--include`, `--exclude`, `--remediation-file`).
- **Repository Filtering:** `--filter-name`, `--filter-language`, `--filter-topics`, `--filter-updated`, `--filter-skip-forks`

### Examples

**Basic Analysis**
Quick analysis with standard depth (100 PRs, 200 issues, 100 workflow runs).

```bash
gh-inspect run owner/repo
```

**Shallow Scan (Fast)**
Minimal API usage for quick checks (50 PRs, 100 issues, 50 workflow runs).

```bash
gh-inspect run owner/repo --depth=shallow
```

**Deep Scan (Comprehensive)**
Thorough analysis with extensive pagination (500 PRs, 1000 issues, 500 workflow runs).

```bash
gh-inspect run owner/repo --depth=deep --since=90d
```

**Custom Depth Limits**
Fine-tune API usage for specific needs.

```bash
# Standard depth but only 25 PRs
gh-inspect run owner/repo --depth=standard --max-prs=25

# Deep scan but limit workflow runs to save API calls
gh-inspect run owner/repo --depth=deep --max-workflow-runs=200
```

**Score Explanation**
Understand what's affecting your health score.

```bash
gh-inspect run owner/repo --explain
```

**Baseline & Regression Detection**
Track score changes over time and catch regressions.

```bash
# First run - establish baseline
gh-inspect run owner/repo --save-baseline

# Later runs - compare against baseline
gh-inspect run owner/repo --compare-last

# Compare against the latest 5 snapshots and print a trend summary
gh-inspect run owner/repo --compare-history=5

# Fail CI if score dropped
gh-inspect run owner/repo --compare-last --fail-on-regression

# Save custom baseline file
gh-inspect run owner/repo --save-baseline --baseline=./baseline-prod.json

# Compare against specific baseline
gh-inspect run owner/repo --baseline=./baseline-prod.json

# Save latest baseline and a timestamped historical snapshot
# Snapshots are stored next to the baseline in a *-history directory
gh-inspect run owner/repo --save-baseline --baseline=./baseline-prod.json
```

**Markdown Output for GitHub Actions**
Generate rich reports for PR comments and Actions summaries.

```bash
gh-inspect run owner/repo --format=markdown --explain > report.md
```

**JSON Output**
Useful for piping into other tools like `jq`.

```bash
gh-inspect run owner/repo --format=json > report.json
```

**Output Modes**
Control how findings are presented to match your workflow:

```bash
# Observational (default) - Neutral facts and metrics
gh-inspect run owner/repo

# Suggestive - Prescriptive advice and recommendations
gh-inspect run owner/repo --output-mode=suggestive

# Statistical - Numbers only, minimal text
gh-inspect run owner/repo --output-mode=statistical

# Save output mode preference to config
gh-inspect config set global.output_mode suggestive
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

Filter which repositories to analyze based on multiple criteria.

```bash
# Only analyze Go and Python repositories
gh-inspect org my-org --filter-language=go,python

# Filter by name pattern (regex)
gh-inspect org my-org --filter-name="^api-.*"

# Only production repositories updated in last 90 days
gh-inspect org my-org --filter-topics=production --filter-updated=90d

# Skip forked repositories
gh-inspect org my-org --filter-skip-forks

# Combine multiple filters
gh-inspect user john --filter-language=typescript --filter-topics=web --filter-updated=30d
```

**Available Analyzers:**

- `activity` - Commit patterns, contributors, bus factor, and code quality metrics
- `prflow` - Pull request velocity, cycle time, review, and collaboration metrics
- `ci` - CI/CD workflow success rates
- `issues` - Issue hygiene and zombie detection
- `security` - Security advisories and vulnerabilities
- `releases` - Release frequency, deployment metrics, and versioning patterns
- `branches` - Branch protection and stale branches
- `dependencies` - Dependency management and package analysis
- `health` - Repository health files (README, LICENSE, etc.)

**Verbose Mode**
Get detailed progress information during long-running analyses.

```bash
gh-inspect run owner/org --verbose
```

### GitHub Actions Integration

Use `gh-inspect` in your GitHub Actions workflows for automated repository health monitoring. The tool automatically integrates with GitHub's step summary when running in Actions.

**Markdown Output Features**

Generate markdown reports with rich formatting, suitable for PR comments and GitHub summaries:

- **Score badges** with color-coded emojis (🟢 90+, 🟡 75-89, 🟠 50-74, 🔴 <50)
- **Collapsible findings** grouped by analyzer for easy navigation
- **Detailed explanations** for each finding with "Why this matters"
- **Actionable suggestions** with numbered steps for improvement
- **Key metrics tables** organized by category
- **Recommendations section** with prioritized insights

```bash
gh-inspect run owner/repo --format=markdown --explain > report.md
```

When running in GitHub Actions with `--format=markdown`, the output is automatically written to `$GITHUB_STEP_SUMMARY` for enhanced visibility.

**Example Workflow with All Features**

Create `.github/workflows/health-check.yml`:

```yaml
name: Repository Health Check

on:
  schedule:
    - cron: "0 9 * * 1" # Weekly on Mondays
  workflow_dispatch:
  pull_request:

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Install gh-inspect
        run: |
          curl -sfL https://raw.githubusercontent.com/mikematt33/gh-inspect/main/install.sh | sh
          echo "$PWD/bin" >> $GITHUB_PATH

      - name: Run analysis with baseline comparison
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          # Shallow scan for PRs, deep analysis for main
          DEPTH="${{ github.event_name == 'pull_request' && 'shallow' || 'deep' }}"

          gh-inspect run ${{ github.repository }} \
            --format=markdown \
            --explain \
            --depth=$DEPTH \
            --compare-last \
            --fail-on-regression \
            --fail-under=70 \
            --save-baseline

      - name: Upload baseline artifact
        if: github.ref == 'refs/heads/main'
        uses: actions/upload-artifact@v4
        with:
          name: health-baseline
          path: ~/.gh-inspect/baseline.json

      - name: Comment on PR
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const report = fs.readFileSync('report.md', 'utf8');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: report
            });
```

See the complete [example workflow](.github/workflows/health-check.yml) for more details.

## 🚀 Enterprise Scaling & Rate Limiting

`gh-inspect` is designed to be highly adaptable, scaling effortlessly from analyzing a single repository to thousands of repositories across enterprise organizations.

### Concurrency Controls

When analyzing organizations that have hundreds or thousands of repositories, the CLI regulates API pressure and memory usage using worker pools governed by a concurrency limit.
By default, concurrency is set to `5`. This limit can be adjusted in the `config.yaml` using the `global.concurrency` field. For massive organizations with high token throughput, significantly increasing this limit can speed up processing pipelines safely without risking local memory starvation.

### Multi-Token Round-Robin Support

Running heavy inspections over large organizations can frequently strike the GitHub REST/GraphQL API rate limits. To aggressively combat API starvation, `gh-inspect` employs token splitting:
You can provide an array of comma-separated tokens via the `GITHUB_TOKENS` environment variable (e.g. `GITHUB_TOKENS="tokenA,tokenB,tokenC"`).
Under the hood, `gh-inspect` internally provisions a thread-safe round-robin rotator that automatically distributes outgoing requests sequentially across all supplied tokens—drastically extending the total number of allowed API calls across multiple identities before hitting block thresholds.

The [`actions`](#actions---github-actions-analytics) command goes a step further with a **quota-aware** pool: it tracks each token's remaining rate-limit independently, always routes the next request to the credential with the most remaining quota, rotates away on `403`/`429`, and can mix in GitHub App installation tokens alongside PATs. See [Actions Analytics & Multi-Credential Configuration](#actions-analytics--multi-credential-configuration) for setup.

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

- **activity** - Always enabled (core metrics including code quality)
- **pr_flow** - Enabled by default, configurable stale threshold (includes collaboration metrics)
- **issue_hygiene** - Enabled by default, configurable stale/zombie thresholds
- **repo_health** - Enabled by default
- **ci** - Enabled by default
- **security** 🆕 - Enabled by default (gracefully handles missing GHAS)
- **releases** 🆕 - Enabled by default (includes deployment metrics)
- **branches** 🆕 - Enabled by default, configurable stale threshold (90 days)
- **dependencies** 🆕 - Enabled by default (multi-language support)

### Actions Analytics & Multi-Credential Configuration

The `actions` command supports a credential pool and its own defaults in `config.yaml`:

```yaml
global:
  # Single token (comma-separated also accepted)
  github_token: ghp_xxx
  # Pool of PATs for quota-aware rotation
  github_tokens:
    - ghp_aaa
    - ghp_bbb
  # One or more GitHub App installations
  github_apps:
    - name: ci-app
      app_id: 12345
      installation_id: 67890
      private_key_path: ./private-key.pem # or `private_key:` with inline PEM

actions:
  days: 30 # default lookback window
  max_runs: 1000 # cap on runs fetched per repo
  sample_job_runs: 0 # recent runs per workflow to inspect for job/runner detail
  duration_threshold_sec: 1800 # flag workflows whose avg duration exceeds this
  queue_threshold_sec: 300 # flag workflows with long queue waits
  confirm_threshold: 1000 # est. API calls above which --confirm is required
```

### Output Modes

gh-inspect offers three output modes to control how findings and recommendations are presented:

#### Observational (Default)

Presents neutral, factual observations about your repository:

- Focus on "what is" rather than "what should be"
- Metrics and facts without prescriptive language
- Ideal for teams that prefer to draw their own conclusions

#### Suggestive

Provides prescriptive advice and actionable recommendations:

- Findings include "Why this matters" explanations
- Specific action items for improvement
- Best for teams seeking guidance on best practices

#### Statistical

Shows metrics and numbers with minimal explanatory text:

- Compact output focused on quantitative data
- Perfect for dashboards and programmatic analysis
- Reduces noise for data-driven workflows

**Usage:**

```bash
# Set via command-line flag
gh-inspect run owner/repo --output-mode=suggestive

# Or save preference to config
gh-inspect config set global.output_mode suggestive

# Flag always overrides config setting
gh-inspect run owner/repo --output-mode=statistical
```

## 🔍 Included Analyzers

gh-inspect includes 9 comprehensive analyzers that examine different aspects of your repository health:

| Analyzer            | Description                      | Key Metrics                                                                     |
| ------------------- | -------------------------------- | ------------------------------------------------------------------------------- |
| **Activity**        | Contributor engagement & growth  | Bus Factor, Stars/Forks, New Contributors, Commit Velocity, Code Quality        |
| **PR Flow**         | Review velocity & quality        | Cycle Time, Self-Merge Rate, Draft Adoption, Description Quality, Collaboration |
| **Issue Hygiene**   | Backlog health & responsiveness  | Time to First Response, Assignee Coverage, Bug/Feature Ratio                    |
| **Repo Health**     | Governance & best practices      | Branch Protection, Dependency Management, Key Files (LICENSE, etc.)             |
| **CI Stability**    | Build health & reliability       | Success Rate, Workflow Cost, Average Runtime                                    |
| **Security** 🆕     | Vulnerability & secret detection | Dependabot Alerts, Secret Scanning, Code Scanning                               |
| **Releases** 🆕     | Release & deployment management  | Release Frequency, Deployment Metrics, Changelog Coverage, Semantic Versioning  |
| **Branches** 🆕     | Branch management                | Total Branches, Stale Branches, Branch Health                                   |
| **Dependencies** 🆕 | Dependency management            | Package Managers, Dependency Counts, Lock Files, Version Pinning                |

### Analyzer Details

#### Activity Analyzer

Tracks contributor engagement, repository popularity, and code quality:

- **Commits Total** - Number of commits in the analysis window
- **Commit Velocity** - Average commits per day
- **Bus Factor** - Number of authors accounting for 50% of commits
- **Active Contributors** - Total distinct commit authors
- **New Contributors** 🆕 - First-time contributors in the window
- **Stars** 🆕 - Repository star count
- **Forks** 🆕 - Repository fork count
- **Watchers** 🆕 - Repository watchers count
- **Code Churn Ratio** 🆕 - Ratio of additions to deletions in PRs
- **Review Coverage** 🆕 - Percentage of PRs that received reviews
- **Merge Without Review Rate** 🆕 - PRs merged without any reviews
- **Avg Review Depth** 🆕 - Average number of review comments per PR

#### PR Flow Analyzer

Analyzes pull request efficiency, quality, and collaboration:

- **Avg Cycle Time** - Time from PR creation to merge
- **Avg Time to First Review** 🆕 - How quickly PRs get initial feedback
- **Avg Approvals per PR** 🆕 - Review engagement level
- **Merge Ratio** - Percentage of PRs that get merged
- **Self-Merge Rate** 🆕 - PRs merged by their own author
- **Draft PR Rate** 🆕 - Adoption of draft PR workflow
- **Description Quality** 🆕 - PRs with meaningful descriptions
- **Avg PR Size** - Lines changed per PR
- **Unique Reviewers** 🆕 - Distinct code reviewers actively participating
- **Avg Reviewers per PR** 🆕 - Average number of reviewers assigned per PR
- **Cross-Author Collaboration** 🆕 - Average reviewers per unique author
- **PR Discussion Depth** 🆕 - Average comments per PR
- **Review Participation** 🆕 - Count of active reviewers in the window

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

Tracks release management practices and deployment patterns:

- **Releases in Window** - Number of releases created
- **Release Frequency** - Average releases per month
- **Avg Days Between Releases** - Release cadence
- **Changelog Coverage** - Releases with notes
- **Semver Compliance** - Semantic versioning adoption
- **Pre-release Ratio** - Beta vs stable releases
- **Days Since Last Release** 🆕 - Time elapsed since most recent release
- **Release Consistency** 🆕 - Coefficient of variation for release cadence (lower is more consistent)
- **Rapid Releases** 🆕 - Count of releases within 2 hours (potential hotfixes)
- **Stable Releases** 🆕 - Count of non-prerelease versions

#### Branches Analyzer 🆕

Monitors branch management hygiene:

- **Total Branches** - All branches in repository
- **Stale Branches** - Branches inactive beyond threshold (default: 90 days)
- Flags repositories with too many branches (>50)
- Identifies cleanup opportunities

#### Dependencies Analyzer 🆕

Analyzes dependency management across multiple languages:

- **Package Managers** - Detected package managers (npm, yarn, pnpm, go-modules, pip, pipenv, poetry, cargo, maven, gradle, bundler, composer, nuget)
- **Total Dependencies** - Aggregate dependency count across all languages
- **Language-Specific Counts** - npm_dependencies, go_dependencies, python_dependencies, rust_dependencies
- **NPM Dev Dependencies** - Development-only npm packages
- **Python Pinned Versions** - Percentage of Python dependencies with pinned versions
- **Lock Files** - Detected lock files (package-lock.json, yarn.lock, Pipfile.lock, Cargo.lock, etc.)

**Findings:**

- **No Dependency Management** - No package manager detected
- **Unpinned Dependencies** - Python dependencies without version pins
- **Dependency Bloat** - Projects with >100 total dependencies
- **Missing Lock File** - No lock file detected for reproducible builds

**Supported Languages:**

- JavaScript/TypeScript (npm, yarn, pnpm)
- Go (go.mod)
- Python (requirements.txt, Pipfile, pyproject.toml)
- Rust (Cargo.toml)
- Java (pom.xml, build.gradle)
- Ruby (Gemfile)
- PHP (composer.json)
- C# (packages.config, .csproj)

All analyzers work with `run`, `org`, `user`, and `compare` commands!

## 💡 Recommendations Engine

Every finding includes rich, actionable recommendations to help you improve your repository health.

### What You Get

**Explanations:**
Each finding explains _why_ it matters, not just what the problem is.

```
🚨 bus_factor_risk: Single contributor risk: 50% of commits are by 1 person
   Why: A bus factor of 1 means that if your primary contributor is unavailable,
        development could stall. This creates single points of failure for your project.
```

**Suggested Actions:**
1-2 concrete, specific steps you can take immediately.

```
   Actions:
   1. Set up pair programming sessions for knowledge transfer
   2. Rotate code review responsibilities across team members
```

### Coverage

Recommendations are provided for:

- **Bus Factor Risk** - Knowledge sharing strategies
- **Stale PRs** - Review process improvements
- **Giant PRs** - Code organization tips
- **Missing Files** - Documentation templates and guides
- **CI Failures** - Debugging and hotfix workflows
- **Branch Protection** - Security configuration steps
- **Slow Builds** - Performance optimization techniques
- **CI Instability** - Test reliability improvements

All recommendations appear in:

- Text output (terminal)
- JSON output (programmatic access)
- Markdown output (GitHub Actions, PR comments)

## ⚡ Performance & API Optimization

gh-inspect is designed to provide comprehensive analysis while minimizing API calls and respecting GitHub's rate limits.

### Smart API Call Management

**Depth Control:**

Choose the right analysis depth for your needs:

- **Shallow** (`--depth=shallow`): 50 PRs, 100 issues, 50 workflow runs
  - Best for: Quick health checks, CI pipelines, frequent monitoring
  - API cost: ~15-25 calls per repository
- **Standard** (`--depth=standard`, default): 100 PRs, 200 issues, 100 workflow runs
  - Best for: Regular analysis, balanced insights
  - API cost: ~25-40 calls per repository
- **Deep** (`--depth=deep`): 500 PRs, 1000 issues, 500 workflow runs
  - Best for: Comprehensive audits, quarterly reviews
  - API cost: ~50-100 calls per repository

**Manual Overrides:**

Fine-tune limits for specific scenarios:

```bash
# Standard depth but reduce PR analysis
gh-inspect run owner/repo --depth=standard --max-prs=25

# Deep analysis but limit workflow runs to save API calls
gh-inspect run owner/repo --depth=deep --max-workflow-runs=200
```

**Disk-Based Caching:**
**Time-Windowed Queries:**

- Only fetches data within the specified analysis period (default: 30 days)
- Uses GitHub's built-in filtering to avoid retrieving unnecessary historical data
- Significantly reduces pagination for active repositories

**Intelligent Pagination:**

- Workflow runs: Configurable per depth (50-500), with accurate all-time total from API
- Issues: Configurable per depth (100-1000)
- Pull requests: Configurable per depth (50-500)
- Commits: Time-bounded to analysis window

**Rate Limit Protection:**

- Pre-flight checks estimate API cost based on depth configuration
- Warns if rate limit might be exhausted
- Automatic rate limit monitoring with sleep/retry on exhaustion
- Real-time rate limit display in `auth status` command

### Typical API Cost

For a **moderately active repository** (50-100 commits/week) with default 30d window:

- **Shallow** (`--depth=shallow`): ~15-25 API calls per repository
- **Standard** (`--depth=standard`): ~25-40 API calls per repository
- **Deep** (`--depth=deep`): ~50-100 API calls per repository

**With Caching:** Second runs within 1 hour use 30-50% fewer API calls.

With authentication, you have 5,000 requests/hour, which allows analyzing:

- **Shallow**: 200-300+ repositories in a single scan
- **Standard**: 125-200 repositories in a single scan
- **Deep**: 50-100 repositories in a single scan

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
