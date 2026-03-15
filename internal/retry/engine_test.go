package retry

import (
	"context"
	"testing"
	"time"
)

func TestTryExecute_WithinBudget(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), 1*time.Hour)
	key := AttemptKey("owner", "repo", "ci", "main", 42)
	ctx := context.Background()

	called := false
	retryFn := func(ctx context.Context, owner, repo string, runID int64, strategy string) error {
		called = true
		return nil
	}

	allowed, err := engine.TryExecute(ctx, key, 3, retryFn, "owner", "repo", 42, "rerun-failed-jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected TryExecute to return true with 0 attempts")
	}
	if !called {
		t.Error("expected retryFn to be called")
	}

	count, _ := engine.CurrentAttempts(ctx, key)
	if count != 1 {
		t.Errorf("expected 1 attempt, got %d", count)
	}
}

func TestTryExecute_ExhaustedBudget(t *testing.T) {
	engine := NewEngine(NewMemoryStore(), 1*time.Hour)
	key := AttemptKey("owner", "repo", "ci", "main", 42)
	ctx := context.Background()

	retryFn := func(ctx context.Context, owner, repo string, runID int64, strategy string) error {
		return nil
	}

	// Simulate 3 attempts.
	for i := 0; i < 3; i++ {
		allowed, err := engine.TryExecute(ctx, key, 3, retryFn, "owner", "repo", 42, "rerun-failed-jobs")
		if err != nil {
			t.Fatalf("unexpected error recording attempt: %v", err)
		}
		if !allowed {
			t.Fatalf("expected attempt %d to be allowed", i+1)
		}
	}

	// 4th attempt should be blocked
	called := false
	retryFn4 := func(ctx context.Context, owner, repo string, runID int64, strategy string) error {
		called = true
		return nil
	}
	allowed, err := engine.TryExecute(ctx, key, 3, retryFn4, "owner", "repo", 42, "rerun-failed-jobs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected TryExecute to return false after 3 attempts with max 3")
	}
	if called {
		t.Error("expected retryFn to NOT be called on exhausted budget")
	}
}

func TestAttemptKey_DifferentBranches(t *testing.T) {
	key1 := AttemptKey("owner", "repo", "ci", "main", 42)
	key2 := AttemptKey("owner", "repo", "ci", "develop", 42)

	if key1 == key2 {
		t.Error("expected different keys for different branches")
	}
}

func TestAttemptKey_DifferentWorkflows(t *testing.T) {
	key1 := AttemptKey("owner", "repo", "ci", "main", 42)
	key2 := AttemptKey("owner", "repo", "deploy", "main", 42)

	if key1 == key2 {
		t.Error("expected different keys for different workflows")
	}
}
