package retry

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Store backed by Redis. It uses INCR for atomic counter
// updates and EXPIRE for TTL-based auto-cleanup.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis-backed store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Get returns the current attempt count for a key. Returns 0 if the key
// does not exist.
func (r *RedisStore) Get(ctx context.Context, key string) (int, error) {
	val, err := r.client.Get(ctx, key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}

// Increment atomically increments the counter and returns the new value.
// On the first increment (result == 1) it sets a TTL so the key expires
// automatically, preventing unbounded growth.
func (r *RedisStore) Increment(ctx context.Context, key string, ttl time.Duration) (int, error) {
	val, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	// Set expiry only on first increment to avoid resetting the TTL
	// on subsequent retries.
	if val == 1 && ttl > 0 {
		if err := r.client.Expire(ctx, key, ttl).Err(); err != nil {
			return int(val), err
		}
	}

	return int(val), nil
}
