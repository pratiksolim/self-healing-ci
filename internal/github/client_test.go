package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v70/github"
)

func setupTestServer(t *testing.T, handler http.Handler) (*github.Client, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := github.NewClient(nil)
	baseURL, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = baseURL
	return client, server.Close
}

func TestFetchJobLogs_NoFailedJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/actions/runs/1/jobs", func(w http.ResponseWriter, r *http.Request) {
		resp := github.Jobs{
			TotalCount: github.Ptr(1),
			Jobs: []*github.WorkflowJob{
				{
					ID:         github.Ptr(int64(100)),
					Name:       github.Ptr("build"),
					Conclusion: github.Ptr("success"),
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	gh, cleanup := setupTestServer(t, mux)
	defer cleanup()

	client := NewClient(gh)
	logs, err := client.FetchJobLogs(context.Background(), "owner", "repo", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

func TestRerunFailedJobs(t *testing.T) {
	mux := http.NewServeMux()
	called := false
	mux.HandleFunc("/repos/owner/repo/actions/runs/42/rerun-failed-jobs", func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})

	gh, cleanup := setupTestServer(t, mux)
	defer cleanup()

	client := NewClient(gh)
	err := client.RerunFailedJobs(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("rerun endpoint was not called")
	}
}

func TestRerunWorkflow(t *testing.T) {
	mux := http.NewServeMux()
	called := false
	mux.HandleFunc("/repos/owner/repo/actions/runs/42/rerun", func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})

	gh, cleanup := setupTestServer(t, mux)
	defer cleanup()

	client := NewClient(gh)
	err := client.RerunWorkflow(context.Background(), "owner", "repo", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("rerun endpoint was not called")
	}
}
