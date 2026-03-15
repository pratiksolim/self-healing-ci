package retry

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})

	return NewRedisStore(rdb), mr
}

func TestRedisStore_TryAttemptAndGet(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	key := "test:counter"
	maxAttempts := 3
	ttl := 1 * time.Hour

	// Initially zero.
	val, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if val != 0 {
		t.Errorf("expected 0, got %d", val)
	}

	// Attempt 1.
	allowed, newVal, err := store.TryAttempt(ctx, key, maxAttempts, ttl)
	if err != nil {
		t.Fatalf("TryAttempt: unexpected error: %v", err)
	}
	if !allowed || newVal != 1 {
		t.Errorf("expected allowed=true, val=1, got %v, %d", allowed, newVal)
	}

	// Attempt 2.
	allowed, newVal, err = store.TryAttempt(ctx, key, maxAttempts, ttl)
	if err != nil {
		t.Fatalf("TryAttempt: unexpected error: %v", err)
	}
	if !allowed || newVal != 2 {
		t.Errorf("expected allowed=true, val=2, got %v, %d", allowed, newVal)
	}

	// Attempt 3.
	allowed, newVal, err = store.TryAttempt(ctx, key, maxAttempts, ttl)
	if err != nil {
		t.Fatalf("TryAttempt: unexpected error: %v", err)
	}
	if !allowed || newVal != 3 {
		t.Errorf("expected allowed=true, val=3, got %v, %d", allowed, newVal)
	}

	// Attempt 4 (Exhausted).
	allowed, newVal, err = store.TryAttempt(ctx, key, maxAttempts, ttl)
	if err != nil {
		t.Fatalf("TryAttempt: unexpected error: %v", err)
	}
	if allowed || newVal != 3 {
		t.Errorf("expected allowed=false, val=3, got %v, %d", allowed, newVal)
	}

	// Get should reflect 3.
	val, err = store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}

func TestRedisStore_TTLExpiry(t *testing.T) {
	store, mr := newTestRedisStore(t)
	ctx := context.Background()
	key := "test:expiry"
	maxAttempts := 3
	ttl := 10 * time.Second

	// First attempt with a short TTL.
	_, _, err := store.TryAttempt(ctx, key, maxAttempts, ttl)
	if err != nil {
		t.Fatalf("TryAttempt: unexpected error: %v", err)
	}

	// Key should exist.
	val, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	// Fast-forward past the TTL.
	mr.FastForward(11 * time.Second)

	// Key should have expired.
	val, err = store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after expiry: unexpected error: %v", err)
	}
	if val != 0 {
		t.Errorf("expected 0 after expiry, got %d", val)
	}
}

func TestRedisStore_IndependentKeys(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()

	_, _, err := store.TryAttempt(ctx, "key:a", 3, 1*time.Hour)
	if err != nil {
		t.Fatalf("TryAttempt key:a: %v", err)
	}
	_, _, err = store.TryAttempt(ctx, "key:a", 3, 1*time.Hour)
	if err != nil {
		t.Fatalf("TryAttempt key:a: %v", err)
	}
	_, _, err = store.TryAttempt(ctx, "key:b", 3, 1*time.Hour)
	if err != nil {
		t.Fatalf("TryAttempt key:b: %v", err)
	}

	valA, err := store.Get(ctx, "key:a")
	if err != nil {
		t.Fatalf("Get key:a: %v", err)
	}
	valB, err := store.Get(ctx, "key:b")
	if err != nil {
		t.Fatalf("Get key:b: %v", err)
	}

	if valA != 2 {
		t.Errorf("key:a: expected 2, got %d", valA)
	}
	if valB != 1 {
		t.Errorf("key:b: expected 1, got %d", valB)
	}
}
