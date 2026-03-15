# Self-Healing CI

A GitHub App that automatically retries failed GitHub Actions workflows when failures match user-configured patterns, such as network timeouts, registry rate limits, or transient connection errors.

## Demo
<video src="./assets/demo.mp4" 
       muted 
       autoplay 
       loop 
       style="max-width: 100%;">
</video>

## 1. How It Works

1. A repository installs the GitHub App.
2. When a workflow run **fails**, GitHub sends a `workflow_run` webhook to this service.
3. The server fetches the repository's `.self-healing-ci.yaml` configuration.
4. Failed job logs are downloaded and scanned against retry-able patterns.
5. If a pattern matches and the retry budget allows, the failed jobs (or full workflow) are automatically re-triggered.
6. If the maximum number of retry attempts is exhausted within the cooldown window, a notification is dispatched to a configured Slack channel.

## 2. Tech Stack

- **Language:** Go 1.25+
- **Build System:** Bazel (with bzlmod) / standard `go` toolchain
- **Data Store:** Redis (for atomic retry budget tracking, with TTL auto-cleanup) / In-memory fallback
- **Containerization:** Docker & Docker Compose
- **Integrations:** GitHub Apps API, Slack `chat.postMessage` API

## 3. Example Configuration

Add a `.self-healing-ci.yaml` to the root of any repository where the app is installed to configure custom retry patterns:

```yaml
version: 1

retry:
  max_attempts: 2              # Max retry attempts per workflow run trigger
  cooldown_seconds: 3600       # Cooldown rolling window between retries

retryable_patterns:
  # Network-related failures (common in CI)
  - name: "npm registry timeout"
    pattern: "ETIMEDOUT.*registry\\.npmjs\\.org"
    is_regex: true
    strategy: "rerun-failed-jobs"

  - name: "docker pull rate limit"
    pattern: "toomanyrequests: You have reached your pull rate limit"
    is_regex: false
    strategy: "rerun-all"

  - name: "go module download failure"
    pattern: "dial tcp.*connection refused"
    is_regex: true
    strategy: "rerun-failed-jobs"
```

## 4. Local Setup Guide

### Prerequisites

- Go 1.25+
- Docker and Docker Compose (highly recommended for local Redis)
- Bazel (optional, for Bazel-specific builds)
- A registered [GitHub App](https://docs.github.com/en/apps/creating-github-apps) with `actions: write`, `contents: read` permissions, and `workflow_run` webhook events.

### Configuration

Create a `.env` file in the project root:

```env
GITHUB_APP_ID=your_app_id
GITHUB_PRIVATE_KEY_PATH=private-key.pem
WEBHOOK_SECRET=your_webhook_secret
SLACK_BOT_TOKEN=xoxb-your-slack-bot-token
SLACK_CHANNEL_ID=C1234567890
PORT=8080
REDIS_ADDR=redis:6379            # Use localhost:6379 if running without Docker
RETRY_COOLDOWN_SECONDS=3600
```

Place your GitHub App's private key in the root directory and name it `private-key.pem` (this file is `.gitignore`d).

### With Docker (Recommended)

To spin up both the Go server and a local Redis instance natively:

```bash
docker compose up --build
```

The application will be running and listening for webhooks on port 8080. You can expose this port securely using a tunnel like `ngrok` (`ngrok http 8080`).

### Without Docker

Ensure you have a Redis instance running locally (or comment out `REDIS_ADDR` from your `.env` to seamlessly fall back to the thread-safe in-memory store).

```bash
# Run with Go
go run ./cmd/server

# Or run with Bazel
bazel run //cmd/server:server
```

## 5. Guide to Fork and Deploy for Your Organization

1. **Fork the Repository**: Fork this repository into your organization's GitHub account.
2. **Setup the GitHub App**:
   - Navigate to Organization Settings -> Developer settings -> GitHub Apps -> New GitHub App.
   - Set the Webhook URL to your public-facing deployment URL (e.g., `https://self-healing-ci.your-org.com/webhook`).
   - Generate a Webhook Secret and note it down.
   - Set permissions: **Actions** (Read & write), **Contents** (Read).
   - Subscribe to the **Workflow run** event.
   - Install the App into the desired repositories.
   - Generate a private key (`.pem`) and note the App ID.
3. **Set up Slack App (Optional, for notifications)**:
   - Create a Slack App in your workspace workspace.
   - Give it the `chat:write` OAuth Scope and install it.
   - Note the **Bot User OAuth Token** (`xoxb-...`).
   - Add the App to your desired target channel and get the **Channel ID**.
4. **Deploy**:
   - Use the provided multi-stage `Dockerfile` to build your production image.
   - Deploy the container to your preferred platform (Kubernetes, AWS ECS, Google Cloud Run, etc.).
   - Provision a production Redis instance (e.g., AWS ElastiCache).
   - Safely inject the `.env` variables via your infrastructure's secret manager, ensuring the GitHub private key is securely mounted or passed down.

## 6. Project Structure

```text
├── cmd/server/           # Server entry point and configuration loading
├── internal/
│   ├── config/           # YAML config loading & validation
│   ├── github/           # GitHub App auth + wrapper client
│   ├── analyzer/         # Log pattern matching engine
│   ├── retry/            # Redis / In-Memory atomic retry & budget Engine
│   └── webhook/          # HTTP webhook handler & Slack integrations
├── examples/             # Example `.self-healing-ci.yaml` configurations
├── Dockerfile            # Lean multi-stage Docker build for the server
├── docker-compose.yaml   # Local dev environment with Redis cluster
├── MODULE.bazel          # Bazel module (bzlmod)
└── go.mod                # Go module
```

## License

MIT
