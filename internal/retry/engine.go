// Package retry provides retry decision logic with budget enforcement.
package retry

import (
	"context"
	"fmt"
	"log"
	"sync"

	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
)

// Engine manages retry decisions and execution.
type Engine struct {
	client *ghclient.Client

	mu       sync.Mutex
	attempts map[int64]int // runID → current attempt count
}

// NewEngine creates a new retry Engine.
func NewEngine(client *ghclient.Client) *Engine {
	return &Engine{
		client:   client,
		attempts: make(map[int64]int),
	}
}

// ShouldRetry checks whether the given run is still within its retry budget.
func (e *Engine) ShouldRetry(runID int64, maxAttempts int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.attempts[runID] < maxAttempts
}

// Execute performs the retry using the specified strategy and increments the attempt counter.
func (e *Engine) Execute(ctx context.Context, owner, repo string, runID int64, strategy string) error {
	e.mu.Lock()
	e.attempts[runID]++
	attempt := e.attempts[runID]
	e.mu.Unlock()

	log.Printf("[retry] executing %s for %s/%s run %d (attempt %d)", strategy, owner, repo, runID, attempt)

	switch strategy {
	case "rerun-failed-jobs":
		return e.client.RerunFailedJobs(ctx, owner, repo, runID)
	case "rerun-all":
		return e.client.RerunWorkflow(ctx, owner, repo, runID)
	default:
		return fmt.Errorf("unknown retry strategy: %s", strategy)
	}
}

// CurrentAttempts returns the current attempt count for a run (used in tests).
func (e *Engine) CurrentAttempts(runID int64) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.attempts[runID]
}
