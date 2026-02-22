package httputil

import (
	"fmt"
	"io"
	"net/http"
)

// CheckStatus returns an error if the response status code is not 2xx.
// The prefix is included in the error message for context (e.g. "discord: webhook").
func CheckStatus(resp *http.Response, prefix string) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s returned %d: %s", prefix, resp.StatusCode, ReadSnippet(resp.Body))
	}
	return nil
}

// ReadSnippet reads up to 200 bytes from r for inclusion in error messages.
func ReadSnippet(r io.Reader) string {
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
