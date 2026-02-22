package httputil

import (
	"strings"
	"testing"
)

func TestReadSnippetEmpty(t *testing.T) {
	got := ReadSnippet(strings.NewReader(""))
	if got != "(empty body)" {
		t.Errorf("got %q, want %q", got, "(empty body)")
	}
}

func TestReadSnippetShort(t *testing.T) {
	got := ReadSnippet(strings.NewReader("hello"))
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestReadSnippetTruncates(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := ReadSnippet(strings.NewReader(long))
	if !strings.HasSuffix(got, "...") {
		t.Error("expected trailing ellipsis for long input")
	}
	if len(got) != 203 { // 200 bytes + "..."
		t.Errorf("got length %d, want 203", len(got))
	}
}
