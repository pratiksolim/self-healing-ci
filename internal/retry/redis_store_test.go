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
	return NewRedisStore(rdb), mr
}

func TestRedisStore_IncrementAndGet(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	key := "test:counter"

	// Initially zero.
	val, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if val != 0 {
		t.Errorf("expected 0, got %d", val)
	}

	// Increment to 1.
	newVal, err := store.Increment(ctx, key, 1*time.Hour)
	if err != nil {
		t.Fatalf("Increment: unexpected error: %v", err)
	}
	if newVal != 1 {
		t.Errorf("expected 1, got %d", newVal)
	}

	// Increment to 2.
	newVal, err = store.Increment(ctx, key, 1*time.Hour)
	if err != nil {
		t.Fatalf("Increment: unexpected error: %v", err)
	}
	if newVal != 2 {
		t.Errorf("expected 2, got %d", newVal)
	}

	// Get should reflect 2.
	val, err = store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}
}

func TestRedisStore_TTLExpiry(t *testing.T) {
	store, mr := newTestRedisStore(t)
	ctx := context.Background()
	key := "test:expiry"

	// Increment with a short TTL.
	_, err := store.Increment(ctx, key, 10*time.Second)
	if err != nil {
		t.Fatalf("Increment: unexpected error: %v", err)
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

	_, err := store.Increment(ctx, "key:a", 1*time.Hour)
	if err != nil {
		t.Fatalf("Increment key:a: %v", err)
	}
	_, err = store.Increment(ctx, "key:a", 1*time.Hour)
	if err != nil {
		t.Fatalf("Increment key:a: %v", err)
	}
	_, err = store.Increment(ctx, "key:b", 1*time.Hour)
	if err != nil {
		t.Fatalf("Increment key:b: %v", err)
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
