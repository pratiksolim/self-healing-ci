package retry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v70/github"
	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
)

func setupMockClient(t *testing.T) (*ghclient.Client, *http.ServeMux, func()) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	client := github.NewClient(nil)
	baseURL, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = baseURL
	return ghclient.NewClient(client), mux, server.Close
}

func TestShouldRetry_WithinBudget(t *testing.T) {
	client, _, cleanup := setupMockClient(t)
	defer cleanup()

	engine := NewEngine(client)
	if !engine.ShouldRetry(1, 3) {
		t.Error("expected ShouldRetry to return true for runID with 0 attempts")
	}
}

func TestShouldRetry_ExhaustedBudget(t *testing.T) {
	client, _, cleanup := setupMockClient(t)
	defer cleanup()

	engine := NewEngine(client)
	// Simulate 3 attempts.
	for i := 0; i < 3; i++ {
		engine.mu.Lock()
		engine.attempts[1]++
		engine.mu.Unlock()
	}

	if engine.ShouldRetry(1, 3) {
		t.Error("expected ShouldRetry to return false after 3 attempts with max 3")
	}
}

func TestExecute_RerunFailedJobs(t *testing.T) {
	client, mux, cleanup := setupMockClient(t)
	defer cleanup()

	called := false
	mux.HandleFunc("/repos/owner/repo/actions/runs/42/rerun-failed-jobs", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	engine := NewEngine(client)
	err := engine.Execute(context.Background(), "owner", "repo", 42, "rerun-failed-jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("rerun-failed-jobs endpoint was not called")
	}
	if engine.CurrentAttempts(42) != 1 {
		t.Errorf("expected 1 attempt, got %d", engine.CurrentAttempts(42))
	}
}

func TestExecute_RerunAll(t *testing.T) {
	client, mux, cleanup := setupMockClient(t)
	defer cleanup()

	called := false
	mux.HandleFunc("/repos/owner/repo/actions/runs/42/rerun", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	engine := NewEngine(client)
	err := engine.Execute(context.Background(), "owner", "repo", 42, "rerun-all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("rerun endpoint was not called")
	}
}

func TestExecute_UnknownStrategy(t *testing.T) {
	client, _, cleanup := setupMockClient(t)
	defer cleanup()

	engine := NewEngine(client)
	err := engine.Execute(context.Background(), "owner", "repo", 1, "bad-strategy")
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
}
