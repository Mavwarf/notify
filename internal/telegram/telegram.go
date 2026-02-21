package telegram

import (
	"fmt"
	"net/http"
	"net/url"
)

// Send posts a message to a Telegram chat via the Bot API.
func Send(token, chatID, message string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	return sendTo(endpoint, chatID, message)
}

// sendTo posts a message to the given endpoint. Extracted for testing.
func sendTo(endpoint, chatID, message string) error {
	resp, err := http.PostForm(endpoint, url.Values{
		"chat_id": {chatID},
		"text":    {message},
	})
	if err != nil {
		return fmt.Errorf("telegram: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: API returned %d", resp.StatusCode)
	}
	return nil
}
