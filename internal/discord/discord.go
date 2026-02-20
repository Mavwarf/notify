package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Send posts a message to a Discord channel via webhook URL.
func Send(webhookURL, message string) error {
	body, err := json.Marshal(map[string]string{"content": message})
	if err != nil {
		return fmt.Errorf("discord: marshal: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: webhook returned %d", resp.StatusCode)
	}
	return nil
}
