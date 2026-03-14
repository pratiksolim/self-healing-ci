package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v70/github"
)

// JobLog holds the log content for a single failed job.
type JobLog struct {
	JobName string
	JobID   int64
	Content string
}

// Client wraps a go-github client for the specific API operations we need.
type Client struct {
	gh *github.Client
}

// NewClient creates a Client from an authenticated go-github client.
func NewClient(gh *github.Client) *Client {
	return &Client{gh: gh}
}

// FetchJobLogs lists jobs for a workflow run and downloads logs for each failed job.
func (c *Client) FetchJobLogs(ctx context.Context, owner, repo string, runID int64) ([]JobLog, error) {
	jobs, _, err := c.gh.Actions.ListWorkflowJobs(ctx, owner, repo, runID, &github.ListWorkflowJobsOptions{
		Filter: "latest",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow jobs: %w", err)
	}

	var logs []JobLog
	for _, job := range jobs.Jobs {
		if job.GetConclusion() != "failure" {
			continue
		}

		logURL, _, err := c.gh.Actions.GetWorkflowJobLogs(ctx, owner, repo, job.GetID(), 1)
		if err != nil {
			return nil, fmt.Errorf("failed to get log URL for job %d: %w", job.GetID(), err)
		}

		content, err := downloadLog(ctx, logURL.String())
		if err != nil {
			return nil, fmt.Errorf("failed to download log for job %d: %w", job.GetID(), err)
		}

		logs = append(logs, JobLog{
			JobName: job.GetName(),
			JobID:   job.GetID(),
			Content: content,
		})
	}

	return logs, nil
}

// RerunFailedJobs triggers a rerun of only the failed jobs in a workflow run.
func (c *Client) RerunFailedJobs(ctx context.Context, owner, repo string, runID int64) error {
	_, err := c.gh.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
	if err != nil {
		return fmt.Errorf("failed to rerun failed jobs for run %d: %w", runID, err)
	}
	return nil
}

// RerunWorkflow triggers a full rerun of the entire workflow run.
func (c *Client) RerunWorkflow(ctx context.Context, owner, repo string, runID int64) error {
	_, err := c.gh.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
	if err != nil {
		return fmt.Errorf("failed to rerun workflow run %d: %w", runID, err)
	}
	return nil
}

// FetchRepoConfigFile downloads a file from the repository at the given ref.
func (c *Client) FetchRepoConfigFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error) {
	fileContent, _, _, err := c.gh.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get config file %s: %w", path, err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file content: %w", err)
	}

	return []byte(content), nil
}

// downloadLog fetches log content from a redirect URL.
func downloadLog(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d downloading log", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Truncate very large logs to avoid memory issues (keep last 50KB).
	content := string(body)
	const maxSize = 50 * 1024
	if len(content) > maxSize {
		content = content[len(content)-maxSize:]
		// Find the first newline to avoid partial line.
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = content[idx+1:]
		}
	}

	return content, nil
}
