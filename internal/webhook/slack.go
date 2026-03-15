package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// sendSlackAlert sends a notification to Slack when retry budget is exhausted.
func sendSlackAlert(ctx context.Context, slackToken, channelID, owner, repo string, runID int64, errMessage string) {
	if slackToken == "" || channelID == "" {
		return
	}

	workflowURL := fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", owner, repo, runID)
	text := fmt.Sprintf("🚨 *Self-Healing CI Auto-Retry Exhausted*\n*Repository:* %s/%s\n*Run:* <%s|View Run %d>\n*Reason:* Max attempts reached. Check failures.\n*Matched Error:* ```%s```", owner, repo, workflowURL, runID, errMessage)

	payload := map[string]interface{}{
		"channel": channelID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[slack] failed to marshal payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		log.Printf("[slack] failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+slackToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[slack] failed to send alert: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("[slack] unexpected response status: %d", resp.StatusCode)
	} else {
		log.Printf("[slack] alert sent successfully for run %d", runID)
	}
}
