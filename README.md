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

Requirements: Go 1.21+

```bash
# Clone the repository
git clone https://github.com/mikematt33/gh-inspect.git
cd gh-inspect

# Build using Make
make build

# Verify installation
./bin/gh-inspect --help
```

## ⚡ Quick Start

1.  **Authentication**: Ensure you have a GitHub token set.

    ```bash
    export GITHUB_TOKEN=your_token_here
    # OR if you use the GitHub CLI:
    gh auth login
    ```

    _or configure it directly:_

    ```bash
    gh-inspect config set-token YOUR_TOKEN
    ```

2.  **Run Analysis**:
    Analyze a specific repository.
    _(The CLI will automatically create a configuration file on first run)_

    ```bash
    gh-inspect run owner/repo
    ```

    _Note: If you built from source, use `./bin/gh-inspect` instead of `gh-inspect`._

## ⚙️ Usage Details

### Available Commands

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

#### `org` - Organization Scan

Scan all active repositories in a GitHub organization. Automatically skips archived repositories.

```bash
gh-inspect org organization [flags]
```

**Flags:**

- Uses the same flags as `run` (`--deep`, `--format`, `--since`, `--fail-under`).

#### `user` - User Scan

Analyze all repositories belonging to a specific user.

```bash
gh-inspect user username [flags]
```

**Flags:**

- Uses the same flags as `run` (`--deep`, `--format`, `--since`, `--fail-under`).

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
gh-inspect update
```

#### `uninstall`

Uninstall the CLI from your system.

```bash
gh-inspect uninstall
```

#### `init` & `config`

Initialize or manage configuration. See [Configuration](#%EF%B8%8F-configuration) for details.

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

# Disable a specific analyzer
gh-inspect config set analyzers.ci.enabled false

# Set specific concurrency limit
gh-inspect config set global.concurrency 10
```

## 🔍 Included Analyzers

| Analyzer          | Description               | Key Metrics                                    |
| ----------------- | ------------------------- | ---------------------------------------------- |
| **Activity**      | Contributor engagement    | Bus Factor, Active Contributors, Commit Volume |
| **PR Flow**       | Review velocity & quality | Cycle Time, Unreviewed PRs, Giant PRs          |
| **Issue Hygiene** | Backlog health            | Stale Issues, Triage Time, Zombie Count        |
| **CI Stability**  | Build health              | Success Rate, Workflow Cost, Flakiness         |
| **Repo Health**   | Governance standards      | LICENSE, README, Security Policy, CODEOWNERS   |

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
