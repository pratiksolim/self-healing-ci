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

// ShouldRetry checks whether the given workflow+branch is still within its retry budget.
func (e *Engine) ShouldRetry(ctx context.Context, key string, maxAttempts int) (bool, error) {
	count, err := e.store.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to get attempt count: %w", err)
	}
	return count < maxAttempts, nil
}

// RecordAttempt increments the attempt counter and returns the new count.
func (e *Engine) RecordAttempt(ctx context.Context, key string) (int, error) {
	return e.store.Increment(ctx, key, e.cooldown)
}

// Execute records the attempt and calls the provided retry function.
func (e *Engine) Execute(ctx context.Context, key string, retryFn RetryFunc, owner, repo string, runID int64, strategy string) error {
	attempt, err := e.RecordAttempt(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}
	log.Printf("[retry] executing %s for %s run %d (attempt %d)", strategy, key, runID, attempt)
	return retryFn(ctx, owner, repo, runID, strategy)
}

// CurrentAttempts returns the current attempt count for a key.
func (e *Engine) CurrentAttempts(ctx context.Context, key string) (int, error) {
	return e.store.Get(ctx, key)
}
