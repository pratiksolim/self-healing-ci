package retry

import (
	"context"
	"sync"
	"time"
)

// Store abstracts counter persistence for retry tracking.
// Implementations must be safe for concurrent use.
type Store interface {
	// TryAttempt atomically checks if the retry budget allows another attempt, and if so,
	// increments the counter. It returns true if the attempt is allowed, along with the
	// new attempt count. The ttl is used to set an auto-expiry on the counter so it doesn't
	// grow unbounded.
	TryAttempt(ctx context.Context, key string, maxAttempts int, ttl time.Duration) (bool, int, error)

	// Get returns the current attempt count for a key. Returns 0 if not found.
	Get(ctx context.Context, key string) (int, error)

	// Clear removes the retry tracking for a given key.
	Clear(ctx context.Context, key string) error
}

// MemoryStore is an in-memory Store backed by a map. Useful for tests and
// local development. It supports TTL via access-time pruning on TryAttempt/Get.
type MemoryStore struct {
	mu       sync.Mutex
	attempts map[string]storeEntry
}

type storeEntry struct {
	count     int
	expiresAt time.Time
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		attempts: make(map[string]storeEntry),
	}
}

// TryAttempt atomically checks the budget, increments, and sets TTL.
func (m *MemoryStore) TryAttempt(_ context.Context, key string, maxAttempts int, ttl time.Duration) (bool, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.attempts[key]
	now := time.Now()

	// Handle expiry
	if exists && !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
		entry = storeEntry{}
		exists = false
	}

	if entry.count >= maxAttempts {
		return false, entry.count, nil
	}

	entry.count++
	if ttl > 0 {
		entry.expiresAt = now.Add(ttl)
	}

	m.attempts[key] = entry
	return true, entry.count, nil
}

// Get returns the current attempt count for a key.
func (m *MemoryStore) Get(_ context.Context, key string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.attempts[key]
	if exists && !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		delete(m.attempts, key)
		return 0, nil
	}

	return entry.count, nil
}

// Clear removes a key from the tracking map.
func (m *MemoryStore) Clear(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.attempts, key)
	return nil
}
