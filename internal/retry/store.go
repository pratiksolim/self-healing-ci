// Package retry provides retry decision logic with budget enforcement.
package retry

import (
	"context"
	"sync"
	"time"
)

// Store abstracts counter persistence for retry tracking.
// Implementations must be safe for concurrent use.
type Store interface {
	// Get returns the current attempt count for a key. Returns 0 if not found.
	Get(ctx context.Context, key string) (int, error)

	// Increment atomically increments the counter and returns the new value.
	// On the first increment it sets a TTL for auto-expiry so stale keys
	// are cleaned up automatically.
	Increment(ctx context.Context, key string, ttl time.Duration) (int, error)
}

// MemoryStore is an in-memory Store backed by a map. Useful for tests and
// local development. Counters never expire.
type MemoryStore struct {
	mu       sync.Mutex
	attempts map[string]int
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		attempts: make(map[string]int),
	}
}

// Get returns the current attempt count for a key.
func (m *MemoryStore) Get(_ context.Context, key string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts[key], nil
}

// Increment increments the counter and returns the new value.
// The ttl parameter is ignored for the in-memory store.
func (m *MemoryStore) Increment(_ context.Context, key string, _ time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[key]++
	return m.attempts[key], nil
}
