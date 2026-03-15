// Package webhook handles incoming GitHub webhook events.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/pratiksolim/self-healing-ci/internal/analyzer"
	"github.com/pratiksolim/self-healing-ci/internal/config"
	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
	"github.com/pratiksolim/self-healing-ci/internal/retry"
)

const configFilePath = ".self-healing-ci.yaml"

// Handler processes GitHub webhook events for workflow_run completions.
type Handler struct {
	auth          *ghclient.AppAuth
	retryEngine   *retry.Engine
	webhookSecret string
}

// NewHandler creates a new webhook Handler with a shared retry engine.
func NewHandler(auth *ghclient.AppAuth, retryEngine *retry.Engine, webhookSecret string) *Handler {
	return &Handler{
		auth:          auth,
		retryEngine:   retryEngine,
		webhookSecret: webhookSecret,
	}
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Validate webhook signature.
	if h.webhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !h.verifySignature(body, sig) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Only process workflow_run events.
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "workflow_run" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ignored event type: %s", eventType)
		return
	}

	var event workflowRunEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "failed to parse event", http.StatusBadRequest)
		return
	}

	// Only process completed + failed runs.
	if event.Action != "completed" || event.WorkflowRun.Conclusion != "failure" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "skipped: action=%s conclusion=%s", event.Action, event.WorkflowRun.Conclusion)
		return
	}

	log.Printf("[webhook] processing failed workflow run %d (%s) for %s on branch %s",
		event.WorkflowRun.ID, event.WorkflowRun.Name, event.Repository.FullName, event.WorkflowRun.HeadBranch)

	// Process asynchronously to avoid holding the webhook response.
	go h.processFailedRun(event)

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, "accepted")
}

func (h *Handler) processFailedRun(event workflowRunEvent) {
	ctx := context.Background()

	owner, repo, err := parseFullName(event.Repository.FullName)
	if err != nil {
		log.Printf("[webhook] error: %v", err)
		return
	}

	// Create a per-installation GitHub client.
	client := ghclient.NewClient(h.auth.ClientForInstallation(event.Installation.ID))

	// Fetch the config file from the repository.
	configBytes, err := client.FetchRepoConfigFile(ctx, owner, repo, event.WorkflowRun.HeadBranch, configFilePath)
	if err != nil {
		log.Printf("[webhook] no config file found in %s/%s: %v", owner, repo, err)
		return
	}

	cfg, err := config.LoadConfig(strings.NewReader(string(configBytes)))
	if err != nil {
		log.Printf("[webhook] invalid config in %s/%s: %v", owner, repo, err)
		return
	}

	// Fetch logs for failed jobs.
	logs, err := client.FetchJobLogs(ctx, owner, repo, event.WorkflowRun.ID)
	if err != nil {
		log.Printf("[webhook] failed to fetch job logs: %v", err)
		return
	}

	if len(logs) == 0 {
		log.Printf("[webhook] no failed job logs found for run %d", event.WorkflowRun.ID)
		return
	}

	// Analyze logs against patterns.
	a := analyzer.NewAnalyzer(cfg.RetryablePatterns)
	match := a.Analyze(logs)
	if match == nil {
		log.Printf("[webhook] no retryable pattern matched for run %d", event.WorkflowRun.ID)
		return
	}

	log.Printf("[webhook] pattern %q matched in job %q: %s", match.PatternName, match.JobName, match.MatchedLine)

	// Build a stable key for retry tracking (survives across re-runs).
	attemptKey := retry.AttemptKey(owner, repo, event.WorkflowRun.Name, event.WorkflowRun.HeadBranch)

	// Build the retry function using the per-installation client.
	retryFn := func(ctx context.Context, owner, repo string, runID int64, strategy string) error {
		switch strategy {
		case "rerun-failed-jobs":
			return client.RerunFailedJobs(ctx, owner, repo, runID)
		case "rerun-all":
			return client.RerunWorkflow(ctx, owner, repo, runID)
		default:
			return fmt.Errorf("unknown retry strategy: %s", strategy)
		}
	}

	// Try checking budget and executing the retry atomically.
	allowed, err := h.retryEngine.TryExecute(
		ctx, attemptKey, cfg.Retry.MaxAttempts,
		retryFn, owner, repo, event.WorkflowRun.ID, match.Strategy,
	)
	if err != nil {
		log.Printf("[webhook] error during retry execution for %s: %v", attemptKey, err)
		// If it was allowed but failed during execution, we don't return early here
		// since we want to print success if it succeeded, but we just return to stop.
		return
	}
	if !allowed {
		log.Printf("[webhook] retry budget exhausted for %s (max %d)", attemptKey, cfg.Retry.MaxAttempts)
		return
	}

	log.Printf("[webhook] successfully triggered %s for run %d", match.Strategy, event.WorkflowRun.ID)
}

func (h *Handler) verifySignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(payload)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}

func parseFullName(fullName string) (string, string, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository full name: %s", fullName)
	}
	return parts[0], parts[1], nil
}

// workflowRunEvent represents the relevant fields from a workflow_run webhook event.
type workflowRunEvent struct {
	Action      string `json:"action"`
	WorkflowRun struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
		HeadBranch string `json:"head_branch"`
	} `json:"workflow_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
}
