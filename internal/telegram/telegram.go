package telegram

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
		return fmt.Errorf("telegram: API returned %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	return nil
}

// SendAudio uploads a WAV file to a Telegram chat via the Bot API.
// The caption is sent as text alongside the audio file.
func SendAudio(token, chatID, wavPath, caption string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendAudio", token)
	return sendAudioTo(endpoint, chatID, wavPath, caption)
}

// sendAudioTo uploads a WAV file to the given endpoint. Extracted for testing.
func sendAudioTo(endpoint, chatID, wavPath, caption string) error {
	f, err := os.Open(wavPath)
	if err != nil {
		return fmt.Errorf("telegram: open wav: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// chat_id field.
	if err := w.WriteField("chat_id", chatID); err != nil {
		return fmt.Errorf("telegram: write chat_id field: %w", err)
	}

	// caption field.
	if err := w.WriteField("caption", caption); err != nil {
		return fmt.Errorf("telegram: write caption field: %w", err)
	}

	// Attach the WAV file.
	part, err := w.CreateFormFile("audio", filepath.Base(wavPath))
	if err != nil {
		return fmt.Errorf("telegram: create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("telegram: copy wav data: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("telegram: close multipart: %w", err)
	}

	resp, err := http.Post(endpoint, w.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("telegram: post audio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: audio API returned %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	return nil
}

// readSnippet reads up to 200 bytes from r for inclusion in error messages.
func readSnippet(r io.Reader) string {
	buf := make([]byte, 200)
	n, _ := io.ReadFull(r, buf)
	if n == 0 {
		return "(empty body)"
	}
	s := string(buf[:n])
	if n == 200 {
		s += "..."
	}
	return s
}
