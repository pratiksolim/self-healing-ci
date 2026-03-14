// Package analyzer provides log analysis against configurable failure patterns.
package analyzer

import (
	"strings"

	"github.com/pratiksolim/self-healing-ci/internal/config"
	ghclient "github.com/pratiksolim/self-healing-ci/internal/github"
)

// MatchResult describes which pattern matched in which job log.
type MatchResult struct {
	PatternName string
	MatchedLine string
	JobName     string
	Strategy    string
}

// Analyzer scans job logs against a set of retryable patterns.
type Analyzer struct {
	patterns []config.RetryablePattern
}

// NewAnalyzer creates an Analyzer from pre-validated and compiled patterns.
func NewAnalyzer(patterns []config.RetryablePattern) *Analyzer {
	return &Analyzer{patterns: patterns}
}

// Analyze scans the provided job logs line-by-line against all patterns.
// Returns the first match found, or nil if no pattern matches.
func (a *Analyzer) Analyze(logs []ghclient.JobLog) *MatchResult {
	for _, log := range logs {
		lines := strings.Split(log.Content, "\n")
		for _, line := range lines {
			for _, p := range a.patterns {
				if a.matches(p, line) {
					return &MatchResult{
						PatternName: p.Name,
						MatchedLine: line,
						JobName:     log.JobName,
						Strategy:    p.Strategy,
					}
				}
			}
		}
	}
	return nil
}

func (a *Analyzer) matches(p config.RetryablePattern, line string) bool {
	if p.IsRegex {
		re := p.CompiledRegexp()
		if re == nil {
			return false
		}
		return re.MatchString(line)
	}
	return strings.Contains(line, p.Pattern)
}
