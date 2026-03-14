package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifySignature_Valid(t *testing.T) {
	secret := "test-secret"
	h := &Handler{webhookSecret: secret}

	payload := []byte(`{"action":"completed"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !h.verifySignature(payload, sig) {
		t.Error("expected valid signature to pass")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	h := &Handler{webhookSecret: "test-secret"}
	payload := []byte(`{"action":"completed"}`)

	if h.verifySignature(payload, "sha256=invalid") {
		t.Error("expected invalid signature to fail")
	}
}

func TestVerifySignature_MissingPrefix(t *testing.T) {
	h := &Handler{webhookSecret: "test-secret"}
	payload := []byte(`{"action":"completed"}`)

	if h.verifySignature(payload, "noprefixhere") {
		t.Error("expected missing prefix to fail")
	}
}

func TestServeHTTP_WrongMethod(t *testing.T) {
	h := NewHandler(nil, "secret")
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestServeHTTP_IgnoredEvent(t *testing.T) {
	h := NewHandler(nil, "")
	body := `{"action":"opened"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestServeHTTP_SkipNonFailure(t *testing.T) {
	h := NewHandler(nil, "")
	event := workflowRunEvent{Action: "completed"}
	event.WorkflowRun.Conclusion = "success"
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "workflow_run")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestParseFullName(t *testing.T) {
	owner, repo, err := parseFullName("octocat/hello-world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "octocat" || repo != "hello-world" {
		t.Errorf("expected octocat/hello-world, got %s/%s", owner, repo)
	}
}

func TestParseFullName_Invalid(t *testing.T) {
	_, _, err := parseFullName("invalid")
	if err == nil {
		t.Fatal("expected error for invalid full name")
	}
}
