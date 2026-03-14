package config

import (
	"strings"
	"testing"
)

func TestLoadConfig_Valid(t *testing.T) {
	yaml := `
version: 1
retry:
  max_attempts: 3
  cooldown_seconds: 30
retryable_patterns:
  - name: "npm timeout"
    pattern: "ETIMEDOUT.*registry.npmjs.org"
    is_regex: true
    strategy: "rerun-failed-jobs"
  - name: "docker rate limit"
    pattern: "toomanyrequests"
    is_regex: false
    strategy: "rerun-all"
`
	cfg, err := LoadConfig(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected max_attempts 3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.CooldownSeconds != 30 {
		t.Errorf("expected cooldown_seconds 30, got %d", cfg.Retry.CooldownSeconds)
	}
	if len(cfg.RetryablePatterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(cfg.RetryablePatterns))
	}

	// Verify regex pattern is compiled.
	p0 := cfg.RetryablePatterns[0]
	if p0.CompiledRegexp() == nil {
		t.Error("expected compiled regex for pattern 0")
	}
	if p0.Strategy != "rerun-failed-jobs" {
		t.Errorf("expected strategy rerun-failed-jobs, got %s", p0.Strategy)
	}

	// Verify literal pattern has no compiled regex.
	p1 := cfg.RetryablePatterns[1]
	if p1.CompiledRegexp() != nil {
		t.Error("expected nil compiled regex for literal pattern")
	}
	if p1.Strategy != "rerun-all" {
		t.Errorf("expected strategy rerun-all, got %s", p1.Strategy)
	}
}

func TestLoadConfig_DefaultStrategy(t *testing.T) {
	yaml := `
version: 1
retry:
  max_attempts: 1
  cooldown_seconds: 10
retryable_patterns:
  - name: "generic failure"
    pattern: "connection refused"
    is_regex: false
`
	cfg, err := LoadConfig(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RetryablePatterns[0].Strategy != DefaultStrategy {
		t.Errorf("expected default strategy %s, got %s", DefaultStrategy, cfg.RetryablePatterns[0].Strategy)
	}
}

func TestLoadConfig_InvalidVersion(t *testing.T) {
	yaml := `
version: 99
retry:
  max_attempts: 1
  cooldown_seconds: 10
retryable_patterns:
  - name: "test"
    pattern: "error"
    is_regex: false
`
	_, err := LoadConfig(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestLoadConfig_EmptyPatterns(t *testing.T) {
	yaml := `
version: 1
retry:
  max_attempts: 1
  cooldown_seconds: 10
retryable_patterns: []
`
	_, err := LoadConfig(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("expected error for empty patterns")
	}
}

func TestLoadConfig_InvalidRegex(t *testing.T) {
	yaml := `
version: 1
retry:
  max_attempts: 1
  cooldown_seconds: 10
retryable_patterns:
  - name: "bad regex"
    pattern: "[invalid"
    is_regex: true
`
	_, err := LoadConfig(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestLoadConfig_InvalidStrategy(t *testing.T) {
	yaml := `
version: 1
retry:
  max_attempts: 1
  cooldown_seconds: 10
retryable_patterns:
  - name: "test"
    pattern: "error"
    is_regex: false
    strategy: "unknown-strategy"
`
	_, err := LoadConfig(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}
