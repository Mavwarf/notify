package telegram

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Mavwarf/notify/internal/httputil"
)

// Send posts a message to a Telegram chat via the Bot API.
func Send(token, chatID, message string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	return sendTo(endpoint, chatID, message)
}

// sendTo posts a message to the given endpoint. Extracted for testing.
func sendTo(endpoint, chatID, message string) error {
	resp, err := httputil.PostForm(endpoint, url.Values{
		"chat_id": {chatID},
		"text":    {message},
	})
	if err != nil {
		return fmt.Errorf("telegram: post: %w", err)
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp, "telegram: API")
}

// SendAudio uploads a WAV file to a Telegram chat via the Bot API.
// The caption is sent as text alongside the audio file.
func SendAudio(token, chatID, wavPath, caption string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendAudio", token)
	return sendAudioTo(endpoint, chatID, wavPath, caption)
}

// sendAudioTo uploads a WAV file to the given endpoint. Extracted for testing.
func sendAudioTo(endpoint, chatID, wavPath, caption string) error {
	return sendFile(endpoint, chatID, wavPath, caption, "audio")
}

// SendVoice uploads an OGG/OPUS file to a Telegram chat via the Bot API.
// Renders as a native voice bubble in Telegram clients.
func SendVoice(token, chatID, oggPath, caption string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendVoice", token)
	return sendVoiceTo(endpoint, chatID, oggPath, caption)
}

// sendVoiceTo uploads an OGG file to the given endpoint. Extracted for testing.
func sendVoiceTo(endpoint, chatID, oggPath, caption string) error {
	return sendFile(endpoint, chatID, oggPath, caption, "voice")
}

// sendFile uploads a file to the given endpoint with the specified form field name.
// The contentType is set on the file part (e.g. "audio/ogg" for voice bubbles).
func sendFile(endpoint, chatID, filePath, caption, fieldName string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("telegram: open file: %w", err)
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

	// Attach the file with the correct MIME type.
	ct := mimeForField(fieldName)
	part, err := createFormFileWithType(w, fieldName, filepath.Base(filePath), ct)
	if err != nil {
		return fmt.Errorf("telegram: create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("telegram: copy file data: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("telegram: close multipart: %w", err)
	}

	resp, err := httputil.Post(endpoint, w.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("telegram: post %s: %w", fieldName, err)
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp, fmt.Sprintf("telegram: %s API", fieldName))
}

// mimeForField returns the MIME type for a given form field name.
func mimeForField(fieldName string) string {
	switch fieldName {
	case "voice":
		return "audio/ogg"
	case "audio":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// createFormFileWithType creates a form file part with a specific Content-Type
// instead of the default application/octet-stream.
func createFormFileWithType(w *multipart.Writer, fieldName, fileName, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	h.Set("Content-Type", contentType)
	return w.CreatePart(h)
}

