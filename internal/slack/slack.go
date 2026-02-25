package slack

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Mavwarf/notify/internal/httputil"
)

// Send posts a message to a Slack channel via incoming webhook URL.
func Send(webhookURL, message string) error {
	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return fmt.Errorf("slack: marshal: %w", err)
	}

	resp, err := httputil.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: post: %w", err)
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp, "slack: webhook")
}
