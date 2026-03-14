// Command server starts the self-healing CI webhook server.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
	"github.com/pratiksolim/self-healing-ci/internal/webhook"
)

func main() {
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

	handler := webhook.NewHandler(auth, webhookSecret)

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
