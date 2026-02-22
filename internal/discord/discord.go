package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Mavwarf/notify/internal/httputil"

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
		return fmt.Errorf("discord: webhook returned %d: %s", resp.StatusCode, httputil.ReadSnippet(resp.Body))
	}
	return nil
}

// SendVoice uploads a WAV file to a Discord channel via webhook URL.
// The caption is sent as the message content alongside the attachment.
func SendVoice(webhookURL, wavPath, caption string) error {
	f, err := os.Open(wavPath)
	if err != nil {
		return fmt.Errorf("discord: open wav: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Attach the WAV file.
	part, err := w.CreateFormFile("file", filepath.Base(wavPath))
	if err != nil {
		return fmt.Errorf("discord: create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("discord: copy wav data: %w", err)
	}

	// Attach the caption as payload_json.
	payload, err := json.Marshal(map[string]string{"content": caption})
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}
	if err := w.WriteField("payload_json", string(payload)); err != nil {
		return fmt.Errorf("discord: write payload field: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("discord: close multipart: %w", err)
	}

	resp, err := http.Post(webhookURL, w.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("discord: post voice: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord: voice webhook returned %d: %s", resp.StatusCode, httputil.ReadSnippet(resp.Body))
	}
	return nil
}

