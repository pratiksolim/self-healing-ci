package retry

import (
	"context"
	"testing"
	"time"
)

func TestShouldRetry_WithinBudget(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), 1*time.Hour)
	key := AttemptKey("owner", "repo", "ci", "main")
	ctx := context.Background()

	allowed, err := engine.ShouldRetry(ctx, key, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected ShouldRetry to return true with 0 attempts")
	}
}

func TestShouldRetry_ExhaustedBudget(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), 1*time.Hour)
	key := AttemptKey("owner", "repo", "ci", "main")
	ctx := context.Background()

	// Simulate 3 attempts.
	for i := 0; i < 3; i++ {
		if _, err := engine.RecordAttempt(ctx, key); err != nil {
			t.Fatalf("unexpected error recording attempt: %v", err)
		}
	}

	allowed, err := engine.ShouldRetry(ctx, key, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected ShouldRetry to return false after 3 attempts with max 3")
	}
}

func TestExecute_CallsRetryFunc(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), 1*time.Hour)
	key := AttemptKey("owner", "repo", "ci", "main")
	ctx := context.Background()
	called := false

	retryFn := func(ctx context.Context, owner, repo string, runID int64, strategy string) error {
		called = true
		if strategy != "rerun-failed-jobs" {
			t.Errorf("expected strategy rerun-failed-jobs, got %s", strategy)
		}
		return nil
	}

	err := engine.Execute(ctx, key, retryFn, "owner", "repo", 42, "rerun-failed-jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("retryFn was not called")
	}
	count, err := engine.CurrentAttempts(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 attempt, got %d", count)
	}
}

func TestAttemptKey_DifferentBranches(t *testing.T) {
	key1 := AttemptKey("owner", "repo", "ci", "main")
	key2 := AttemptKey("owner", "repo", "ci", "develop")

	if key1 == key2 {
		t.Error("expected different keys for different branches")
	}
}

func TestAttemptKey_DifferentWorkflows(t *testing.T) {
	key1 := AttemptKey("owner", "repo", "ci", "main")
	key2 := AttemptKey("owner", "repo", "deploy", "main")

	if key1 == key2 {
		t.Error("expected different keys for different workflows")
	}
}
