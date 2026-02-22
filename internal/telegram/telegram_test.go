package telegram

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSendSuccess(t *testing.T) {
	var gotChatID, gotText string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotChatID = r.FormValue("chat_id")
		gotText = r.FormValue("text")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// Override endpoint by passing server URL as token trick:
	// We can't override the real endpoint, so test Send with a custom function.
	// Instead, test the HTTP mechanics via a helper.
	err := sendTo(srv.URL, "123456", "hello world")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotChatID != "123456" {
		t.Errorf("chat_id = %q, want %q", gotChatID, "123456")
	}
	if gotText != "hello world" {
		t.Errorf("text = %q, want %q", gotText, "hello world")
	}
}

func TestSendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := sendTo(srv.URL, "123456", "test")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestSendAudioSuccess(t *testing.T) {
	var gotChatID, gotCaption, gotFilename string
	var gotFileData []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse content type: %v", err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(part)
			switch part.FormName() {
			case "chat_id":
				gotChatID = string(data)
			case "caption":
				gotCaption = string(data)
			case "audio":
				gotFilename = part.FileName()
				gotFileData = data
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tmp := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(tmp, []byte("RIFF fake wav data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := sendAudioTo(srv.URL, "123456", tmp, "audio caption")
	if err != nil {
		t.Fatalf("SendAudio: %v", err)
	}
	if gotChatID != "123456" {
		t.Errorf("chat_id = %q, want %q", gotChatID, "123456")
	}
	if gotCaption != "audio caption" {
		t.Errorf("caption = %q, want %q", gotCaption, "audio caption")
	}
	if gotFilename != "test.wav" {
		t.Errorf("filename = %q, want %q", gotFilename, "test.wav")
	}
	if string(gotFileData) != "RIFF fake wav data" {
		t.Errorf("file data = %q, want %q", gotFileData, "RIFF fake wav data")
	}
}

func TestSendAudioError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()

	tmp := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(tmp, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := sendAudioTo(srv.URL, "123456", tmp, "test")
	if err == nil {
		t.Fatal("expected error for 413 response")
	}
}

func TestSendAudioMissingFile(t *testing.T) {
	err := sendAudioTo("http://example.com", "123456", "/nonexistent/file.wav", "test")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
