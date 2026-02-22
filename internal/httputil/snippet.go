package httputil

import "io"

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
