package telegram

import (
	"net/http"
	"net/http/httptest"
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
