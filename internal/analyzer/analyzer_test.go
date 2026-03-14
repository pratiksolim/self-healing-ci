package analyzer

import (
	"strings"
	"testing"

	"github.com/pratiksolim/self-healing-ci/internal/config"
	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
)

func makePatterns(t *testing.T, raw []config.RetryablePattern) []config.RetryablePattern {
	t.Helper()
	// Use LoadConfig to compile patterns properly via a round-trip.
	// For tests, we'll manually set up compiled patterns.
	return raw
}

func TestAnalyze_RegexMatch(t *testing.T) {
	patterns := []config.RetryablePattern{
		{
			Name:     "npm timeout",
			Pattern:  "ETIMEDOUT.*registry.npmjs.org",
			IsRegex:  true,
			Strategy: "rerun-failed-jobs",
		},
	}
	// Compile via config.LoadConfig round-trip
	cfg := compilePatterns(t, patterns)

	analyzer := NewAnalyzer(cfg)
	logs := []ghclient.JobLog{
		{
			JobName: "install-deps",
			JobID:   1,
			Content: "Step 1: Installing dependencies\nError: ETIMEDOUT connecting to registry.npmjs.org\nStep failed",
		},
	}

	result := analyzer.Analyze(logs)
	if result == nil {
		t.Fatal("expected a match")
	}
	if result.PatternName != "npm timeout" {
		t.Errorf("expected pattern name 'npm timeout', got %q", result.PatternName)
	}
	if result.JobName != "install-deps" {
		t.Errorf("expected job name 'install-deps', got %q", result.JobName)
	}
	if result.Strategy != "rerun-failed-jobs" {
		t.Errorf("expected strategy 'rerun-failed-jobs', got %q", result.Strategy)
	}
}

func TestAnalyze_LiteralMatch(t *testing.T) {
	patterns := []config.RetryablePattern{
		{
			Name:     "docker rate limit",
			Pattern:  "toomanyrequests",
			IsRegex:  false,
			Strategy: "rerun-all",
		},
	}
	cfg := compilePatterns(t, patterns)

	analyzer := NewAnalyzer(cfg)
	logs := []ghclient.JobLog{
		{
			JobName: "docker-build",
			JobID:   2,
			Content: "Pulling image...\ntoomanyrequests: rate limit exceeded\nBuild failed",
		},
	}

	result := analyzer.Analyze(logs)
	if result == nil {
		t.Fatal("expected a match")
	}
	if result.Strategy != "rerun-all" {
		t.Errorf("expected strategy 'rerun-all', got %q", result.Strategy)
	}
}

func TestAnalyze_NoMatch(t *testing.T) {
	patterns := []config.RetryablePattern{
		{
			Name:     "npm timeout",
			Pattern:  "ETIMEDOUT.*registry.npmjs.org",
			IsRegex:  true,
			Strategy: "rerun-failed-jobs",
		},
	}
	cfg := compilePatterns(t, patterns)

	analyzer := NewAnalyzer(cfg)
	logs := []ghclient.JobLog{
		{
			JobName: "test",
			JobID:   3,
			Content: "Running tests...\nTest failed: expected 4 got 5\nDone",
		},
	}

	result := analyzer.Analyze(logs)
	if result != nil {
		t.Errorf("expected no match, got %+v", result)
	}
}

func TestAnalyze_MultiplePatterns_FirstMatchWins(t *testing.T) {
	patterns := []config.RetryablePattern{
		{
			Name:     "pattern-a",
			Pattern:  "first-error",
			IsRegex:  false,
			Strategy: "rerun-failed-jobs",
		},
		{
			Name:     "pattern-b",
			Pattern:  "second-error",
			IsRegex:  false,
			Strategy: "rerun-all",
		},
	}
	cfg := compilePatterns(t, patterns)

	analyzer := NewAnalyzer(cfg)
	logs := []ghclient.JobLog{
		{
			JobName: "job1",
			JobID:   1,
			Content: "line with first-error here\nline with second-error here",
		},
	}

	result := analyzer.Analyze(logs)
	if result == nil {
		t.Fatal("expected a match")
	}
	if result.PatternName != "pattern-a" {
		t.Errorf("expected first pattern to win, got %q", result.PatternName)
	}
}

// compilePatterns is a test helper that uses config.LoadConfig to properly compile regex patterns.
func compilePatterns(t *testing.T, patterns []config.RetryablePattern) []config.RetryablePattern {
	t.Helper()
	import_yaml := "version: 1\nretry:\n  max_attempts: 1\n  cooldown_seconds: 10\nretryable_patterns:\n"
	for _, p := range patterns {
		import_yaml += "  - name: \"" + p.Name + "\"\n"
		import_yaml += "    pattern: \"" + p.Pattern + "\"\n"
		if p.IsRegex {
			import_yaml += "    is_regex: true\n"
		} else {
			import_yaml += "    is_regex: false\n"
		}
		if p.Strategy != "" {
			import_yaml += "    strategy: \"" + p.Strategy + "\"\n"
		}
	}

	cfg, err := config.LoadConfig(strings.NewReader(import_yaml))
	if err != nil {
		t.Fatalf("failed to compile patterns: %v", err)
	}
	return cfg.RetryablePatterns
}
