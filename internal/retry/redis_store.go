package retry

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Store backed by Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis-backed store.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// Lua script to atomically check, increment, and set TTL.
// Keys[1] = key
// ARGV[1] = maxAttempts
// ARGV[2] = ttl (in seconds)
// Returns: {allowed (1/0), currentCount}
var tryAttemptScript = redis.NewScript(`
local count = tonumber(redis.call("GET", KEYS[1]) or "0")
local maxAttempts = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

if count >= maxAttempts then
	return {0, count}
end

local newVal = redis.call("INCR", KEYS[1])
if count == 0 and ttl > 0 then
	redis.call("EXPIRE", KEYS[1], ttl)
end

return {1, newVal}
`)

// TryAttempt atomically uses a Lua script to check budget, increment, and set TTL.
func (r *RedisStore) TryAttempt(ctx context.Context, key string, maxAttempts int, ttl time.Duration) (bool, int, error) {
	ttlSeconds := int(ttl.Seconds())
	res, err := tryAttemptScript.Run(ctx, r.client, []string{key}, maxAttempts, ttlSeconds).Result()
	if err != nil {
		return false, 0, err
	}

	// The Lua script returns a table (slice of interface{})
	slice, ok := res.([]interface{})
	if !ok || len(slice) != 2 {
		return false, 0, errors.New("invalid response from Lua script")
	}

	allowedInt, ok1 := slice[0].(int64)
	countInt, ok2 := slice[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, errors.New("invalid response format from Lua script")
	}

	return allowedInt == 1, int(countInt), nil
}

// Get returns the current attempt count for a key.
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
