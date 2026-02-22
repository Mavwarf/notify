package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Mavwarf/notify/internal/httputil"
)

// Send posts a message to a Slack channel via incoming webhook URL.
func Send(webhookURL, message string) error {
	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return fmt.Errorf("slack: marshal: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: webhook returned %d: %s", resp.StatusCode, httputil.ReadSnippet(resp.Body))
	}
	return nil
}
