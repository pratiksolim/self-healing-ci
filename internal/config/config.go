// Package config provides configuration loading and types for the self-healing CI tool.
package config

import (
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"
)

// DefaultStrategy is used when a pattern does not specify a strategy.
const DefaultStrategy = "rerun-failed-jobs"

// Config is the top-level configuration structure parsed from .self-healing-ci.yaml.
type Config struct {
	Version           int                `yaml:"version"`
	Retry             RetryConfig        `yaml:"retry"`
	RetryablePatterns []RetryablePattern `yaml:"retryable_patterns"`
}

// RetryConfig holds retry budget parameters.
type RetryConfig struct {
	MaxAttempts     int `yaml:"max_attempts"`
	CooldownSeconds int `yaml:"cooldown_seconds"`
}

// RetryablePattern defines a single failure pattern that should trigger a retry.
type RetryablePattern struct {
	Name     string `yaml:"name"`
	Pattern  string `yaml:"pattern"`
	IsRegex  bool   `yaml:"is_regex"`
	Strategy string `yaml:"strategy"`

	// compiled holds the compiled regex if IsRegex is true.
	compiled *regexp.Regexp
}

// CompiledRegexp returns the compiled regular expression for regex patterns.
// Returns nil for literal patterns.
func (p *RetryablePattern) CompiledRegexp() *regexp.Regexp {
	return p.compiled
}

// LoadConfig reads and parses a YAML configuration from the provided reader.
func LoadConfig(r io.Reader) (*Config, error) {
	var cfg Config
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if err := cfg.compilePatterns(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d (expected 1)", c.Version)
	}
	if c.Retry.MaxAttempts < 1 {
		return fmt.Errorf("retry.max_attempts must be >= 1, got %d", c.Retry.MaxAttempts)
	}
	if len(c.RetryablePatterns) == 0 {
		return fmt.Errorf("retryable_patterns must not be empty")
	}
	for i, p := range c.RetryablePatterns {
		if p.Name == "" {
			return fmt.Errorf("retryable_patterns[%d].name must not be empty", i)
		}
		if p.Pattern == "" {
			return fmt.Errorf("retryable_patterns[%d].pattern must not be empty", i)
		}
	}
	return nil
}

func (c *Config) compilePatterns() error {
	for i := range c.RetryablePatterns {
		p := &c.RetryablePatterns[i]

		// Apply default strategy if not set.
		if p.Strategy == "" {
			p.Strategy = DefaultStrategy
		}
		if p.Strategy != "rerun-failed-jobs" && p.Strategy != "rerun-all" {
			return fmt.Errorf("retryable_patterns[%d].strategy must be 'rerun-failed-jobs' or 'rerun-all', got %q", i, p.Strategy)
		}

		if p.IsRegex {
			compiled, err := regexp.Compile(p.Pattern)
			if err != nil {
				return fmt.Errorf("retryable_patterns[%d] has invalid regex %q: %w", i, p.Pattern, err)
			}
			p.compiled = compiled
		}
	}
	return nil
}
