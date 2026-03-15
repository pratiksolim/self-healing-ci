// Command server starts the self-healing CI webhook server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
	"github.com/pratiksolim/self-healing-ci/internal/retry"
	"github.com/pratiksolim/self-healing-ci/internal/webhook"
)

func main() {
	// Load .env file if present (not required — env vars work too).
	if err := godotenv.Load(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("No .env file found, reading from environment")
		} else {
			log.Printf("Warning: error loading .env file: %v", err)
		}
	}

	appIDStr := os.Getenv("GITHUB_APP_ID")
	privateKeyPath := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	port := os.Getenv("PORT")

	if appIDStr == "" || privateKeyPath == "" {
		log.Fatal("GITHUB_APP_ID and GITHUB_PRIVATE_KEY_PATH environment variables are required")
	}

	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		log.Fatalf("invalid GITHUB_APP_ID: %v", err)
	}

	if port == "" {
		port = "8080"
	}

	auth, err := ghclient.NewAppAuth(appID, privateKeyPath)
	if err != nil {
		log.Fatalf("failed to initialize GitHub App auth: %v", err)
	}

	// Configure retry cooldown (how long before counters auto-expire).
	cooldownSeconds := 3600 // default: 1 hour
	if v := os.Getenv("RETRY_COOLDOWN_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err != nil {
			log.Printf("invalid RETRY_COOLDOWN_SECONDS %q: %v. Defaulting to 3600", v, err)
		} else if parsed <= 0 {
			log.Printf("RETRY_COOLDOWN_SECONDS must be > 0. Defaulting to 3600")
		} else {
			cooldownSeconds = parsed
		}
	}
	cooldown := time.Duration(cooldownSeconds) * time.Second

	// Choose store backend: Redis if REDIS_ADDR is set, else in-memory.
	var store retry.Store
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		rdb := redis.NewClient(&redis.Options{Addr: addr})

		// Validate Redis connectivity with a short timeout and fail fast if unavailable.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Fatalf("failed to connect to Redis at %s: %v", addr, err)
		}

		// Ensure the Redis client is cleanly closed on shutdown.
		defer func() {
			if err := rdb.Close(); err != nil {
				log.Printf("error closing Redis client: %v", err)
			}
		}()

		store = retry.NewRedisStore(rdb)
		log.Printf("using Redis store at %s", addr)
	} else {
		store = retry.NewMemoryStore()
		log.Println("REDIS_ADDR not set, using in-memory store (state will be lost on restart)")
	}

	retryEngine := retry.NewEngine(store, cooldown)
	handler := webhook.NewHandler(auth, retryEngine, webhookSecret)

	mux := http.NewServeMux()
	mux.Handle("/webhook", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	addr := ":" + port
	log.Printf("self-healing-ci server starting on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
