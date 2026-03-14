package github

import (
	"testing"
)

func TestNewAppAuth_InvalidKeyPath(t *testing.T) {
	_, err := NewAppAuth(12345, "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent key file")
	}
}

func TestNewAppAuthFromTransport(t *testing.T) {
	auth := NewAppAuthFromTransport(12345, nil)
	if auth == nil {
		t.Fatal("expected non-nil AppAuth")
	}
	if auth.appID != 12345 {
		t.Errorf("expected appID 12345, got %d", auth.appID)
	}
}
