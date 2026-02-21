package discord

import (
	"encoding/json"
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
	var gotBody map[string]string
	var gotContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := Send(srv.URL, "hello world")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["content"] != "hello world" {
		t.Errorf("content = %q, want %q", gotBody["content"], "hello world")
	}
}

func TestSendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	err := Send(srv.URL, "test")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestSendVoiceSuccess(t *testing.T) {
	var gotFilename string
	var gotFileData []byte
	var gotPayload map[string]string

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
			case "file":
				gotFilename = part.FileName()
				gotFileData = data
			case "payload_json":
				json.Unmarshal(data, &gotPayload)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Create a temp WAV file.
	tmp := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(tmp, []byte("RIFF fake wav data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := SendVoice(srv.URL, tmp, "voice caption")
	if err != nil {
		t.Fatalf("SendVoice: %v", err)
	}
	if gotFilename != "test.wav" {
		t.Errorf("filename = %q, want %q", gotFilename, "test.wav")
	}
	if string(gotFileData) != "RIFF fake wav data" {
		t.Errorf("file data = %q, want %q", gotFileData, "RIFF fake wav data")
	}
	if gotPayload["content"] != "voice caption" {
		t.Errorf("payload content = %q, want %q", gotPayload["content"], "voice caption")
	}
}

func TestSendVoiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()

	tmp := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(tmp, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := SendVoice(srv.URL, tmp, "test")
	if err == nil {
		t.Fatal("expected error for 413 response")
	}
}

func TestSendVoiceMissingFile(t *testing.T) {
	err := SendVoice("http://example.com", "/nonexistent/file.wav", "test")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
