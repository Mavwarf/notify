package discord

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Mavwarf/notify/internal/httputil"
)

// Send posts a message to a Discord channel via webhook URL.
func Send(webhookURL, message string) error {
	body, err := json.Marshal(map[string]string{"content": message})
	if err != nil {
		return fmt.Errorf("discord: marshal: %w", err)
	}

	resp, err := httputil.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: post: %w", err)
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp, "discord: webhook")
}

// SendVoice uploads a WAV file to a Discord channel via webhook URL.
// The caption is sent as the message content alongside the attachment.
func SendVoice(webhookURL, wavPath, caption string) error {
	payload, err := json.Marshal(map[string]string{"content": caption})
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}

	resp, err := httputil.PostMultipart(webhookURL, httputil.FileUpload{
		FieldName: "file",
		FilePath:  wavPath,
	}, [][2]string{{"payload_json", string(payload)}})
	if err != nil {
		return fmt.Errorf("discord: post voice: %w", err)
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp, "discord: voice webhook")
}

