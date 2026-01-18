# Contributing to gh-inspect

Thank you for your interest in contributing to `gh-inspect`! We welcome contributions to make repository analysis more insightful and easier to use.

## Development Setup

### Prerequisites

- Go 1.23 or higher
- `make` (optional, but recommended)

### Environment Variables

To avoid hitting GitHub API rate limits immediately, you must provide a Personal Access Token.

You can set it via environment variable:

```bash
export GITHUB_TOKEN="your_token_here"
```

Or configure it in the application:

```bash
go run cmd/gh-inspect/main.go config set-token YOUR_TOKEN
```

### Building the Project

We use a `Makefile` to simplify common tasks.

```bash
# Build the binary to bin/gh-inspect
make build

# Format, vet, test, and build
make all
```

### Running Tests

```bash
make test
```

## Project Structure

- **`cmd/gh-inspect`**: The main entry point and CLI command definitions (using Cobra).
- **`internal/analysis`**: Core business logic.
  - **`analyzers/`**: Individual plugins for specific checks (e.g., `ci`, `prflow`).
- **`internal/github/client.go`**: GitHub API client wrapper with rate limiting logic.
- **`internal/report`**: Renderers for Text or JSON output.
- **`pkg/models`**: Shared structs (`Report`, `RepoResult`, `Metric`).
- **`pkg/insights`**: Intelligence engine for scoring and recommendations.

## Adding a New Analyzer

1. Create a new package in `internal/analysis/analyzers/<name>`.
2. Implement the `Analyzer` interface (`Name()`, `Analyze()`).
3. Register the analyzer in `internal/cli/common.go` inside `RunAnalysisPipeline`.
4. Update `internal/config/config.go` to add configuration structs/defaults.

### Configuration Changes

If you modify the `Config` structs in `internal/config/config.go`, please ensure you:

1. Add `yaml` tags in `snake_case`.
2. This ensures the CLI `config set` command can automatically discover and set the new fields.

## Code Style

- Please run `go fmt ./...` before submitting.
- Ensure your code passes `go vet ./...`.
