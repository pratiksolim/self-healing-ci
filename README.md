# Self-Healing CI

A GitHub App that automatically retries failed GitHub Actions workflows when failures match user-configured patterns — such as network timeouts, registry rate limits, or transient connection errors.

## How It Works

1. A repository installs the GitHub App
2. When a workflow run **fails**, GitHub sends a `workflow_run` webhook
3. The server fetches the repo's `.self-healing-ci.yaml` config
4. Failed job logs are downloaded and scanned against retryable patterns
5. If a pattern matches and retry budget allows, the failed jobs (or full workflow) are automatically re-triggered

## Configuration

Add a `.self-healing-ci.yaml` to the root of any repository where the app is installed:

```yaml
version: 1
retry:
  max_attempts: 2
  cooldown_seconds: 60

retryable_patterns:
  - name: "npm registry timeout"
    pattern: "ETIMEDOUT.*registry\\.npmjs\\.org"
    is_regex: true
    strategy: "rerun-failed-jobs"    # only rerun the failed jobs

  - name: "docker pull rate limit"
    pattern: "toomanyrequests: You have reached your pull rate limit"
    is_regex: false
    strategy: "rerun-all"            # rerun the entire workflow

  - name: "go module download failure"
    pattern: "dial tcp.*connection refused"
    is_regex: true
    # strategy defaults to "rerun-failed-jobs"
```

| Field | Description |
|-------|-------------|
| `retry.max_attempts` | Max retries per workflow run |
| `retry.cooldown_seconds` | Wait time between retries |
| `retryable_patterns[].name` | Human-readable label for the pattern |
| `retryable_patterns[].pattern` | String or regex to match in job logs |
| `retryable_patterns[].is_regex` | `true` for regex, `false` for literal match |
| `retryable_patterns[].strategy` | `rerun-failed-jobs` (default) or `rerun-all` |

See [`examples/.self-healing-ci.yaml`](examples/.self-healing-ci.yaml) for a full example with common CI failure patterns.

## Setup

### Prerequisites

- Go 1.25+
- [Bazel](https://bazel.build/) (with bzlmod support)
- A registered [GitHub App](https://docs.github.com/en/apps/creating-github-apps) with:
  - **Permissions**: `actions: write`, `contents: read`
  - **Webhook events**: `workflow_run`

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GITHUB_APP_ID` | ✅ | Your GitHub App's ID |
| `GITHUB_PRIVATE_KEY_PATH` | ✅ | Path to the app's `.pem` private key |
| `WEBHOOK_SECRET` | Recommended | Webhook secret for signature verification |
| `PORT` | No | Server port (default: `8080`) |

### Run

```bash
# With Go
go run ./cmd/server

# With Bazel
bazel run //cmd/server:server
```

### Build & Test

```bash
# Generate/update BUILD files
bazel run //:gazelle

# Build all targets
bazel build //...

# Run all tests
bazel test //...

# Or with Go directly
go test ./...
```

## Project Structure

```
├── cmd/server/           # Server entry point
├── internal/
│   ├── config/           # YAML config loading & validation
│   ├── github/           # GitHub App auth + API client
│   ├── analyzer/         # Log pattern matching engine
│   ├── retry/            # Retry budget & strategy dispatch
│   └── webhook/          # HTTP webhook handler
├── examples/             # Example configuration
├── MODULE.bazel          # Bazel module (bzlmod)
└── go.mod                # Go module
```

## License

MIT
