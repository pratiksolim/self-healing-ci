// Package retry provides retry decision logic with budget enforcement.
package retry

import (
	"context"
	"fmt"
	"log"
	"time"
)

// RetryFunc is a function that executes the actual retry API call.
// This decouples the engine from the GitHub client, since the client
// is created per-installation and varies per webhook event.
type RetryFunc func(ctx context.Context, owner, repo string, runID int64, strategy string) error

// Engine manages retry decisions and execution.
// It tracks attempts by a stable key (workflow name + branch) rather than
// by run ID, because retries create new runs with different IDs.
type Engine struct {
	store    Store
	cooldown time.Duration
}

// NewEngine creates a new retry Engine backed by the given Store.
// The cooldown duration controls how long retry counters live before
// auto-expiring (used by stores that support TTL, e.g. Redis).
func NewEngine(store Store, cooldown time.Duration) *Engine {
	return &Engine{
		store:    store,
		cooldown: cooldown,
	}
}

// AttemptKey builds a stable key for tracking retries across workflow re-runs.
func AttemptKey(owner, repo, workflowName, branch string) string {
	return fmt.Sprintf("%s/%s:%s:%s", owner, repo, workflowName, branch)
}

// TryExecute records an attempt using atomic budget enforcement.
// If the retry budget allows another attempt, it increments the attempt
// counter, records a log, and then calls the provided retry function.
// It returns true if the budget allowed the attempt regardless of whether
// the retryFn succeeded or failed.
func (e *Engine) TryExecute(
	ctx context.Context,
	key string,
	maxAttempts int,
	retryFn RetryFunc,
	owner, repo string,
	runID int64,
	strategy string,
) (bool, error) {
	allowed, currentAttempt, err := e.store.TryAttempt(ctx, key, maxAttempts, e.cooldown)
	if err != nil {
		return false, fmt.Errorf("failed to check/record attempt: %w", err)
	}

	if !allowed {
		return false, nil
	}

	log.Printf("[retry] executing %s for %s run %d (attempt %d)", strategy, key, runID, currentAttempt)
	
	if err := retryFn(ctx, owner, repo, runID, strategy); err != nil {
		return true, fmt.Errorf("retry execution failed: %w", err)
	}
	
	return true, nil
}

// CurrentAttempts returns the current attempt count for a key.
func (e *Engine) CurrentAttempts(ctx context.Context, key string) (int, error) {
	return e.store.Get(ctx, key)
}
