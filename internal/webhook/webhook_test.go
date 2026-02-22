package webhook

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendSuccess(t *testing.T) {
	var gotBody string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := Send(srv.URL, "hello world", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != "hello world" {
		t.Errorf("body = %q, want %q", gotBody, "hello world")
	}
	if gotContentType != "text/plain" {
		t.Errorf("content-type = %q, want %q", gotContentType, "text/plain")
	}
}

func TestSendCustomHeaders(t *testing.T) {
	t.Setenv("TEST_WEBHOOK_TOKEN", "secret123")

	var gotAuth string
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	headers := map[string]string{
		"Authorization": "Bearer $TEST_WEBHOOK_TOKEN",
		"Content-Type":  "application/json",
	}
	err := Send(srv.URL, `{"msg":"hi"}`, headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret123")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
}

func TestSendErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		io.WriteString(w, "internal server error")
	}))
	defer srv.Close()

	err := Send(srv.URL, "test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code 500: %v", err)
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("error should contain body snippet: %v", err)
	}
}
