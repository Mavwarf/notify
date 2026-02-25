package telegram

import (
	"fmt"
	"net/url"

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
	resp, err := httputil.PostMultipart(endpoint, httputil.FileUpload{
		FieldName:   fieldName,
		FilePath:    filePath,
		ContentType: mimeForField(fieldName),
	}, [][2]string{{"chat_id", chatID}, {"caption", caption}})
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

