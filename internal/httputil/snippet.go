package httputil

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// Client is a shared HTTP client with a 30-second timeout, used by all
// remote step packages to avoid indefinite hangs on unresponsive servers.
var Client = &http.Client{Timeout: 30 * time.Second}

// Post issues a POST using the shared Client.
func Post(url, contentType string, body io.Reader) (*http.Response, error) {
	return Client.Post(url, contentType, body)
}

// PostForm issues a POST with form data using the shared Client.
func PostForm(endpoint string, data url.Values) (*http.Response, error) {
	return Client.PostForm(endpoint, data)
}

// FileUpload describes a file to include in a multipart form POST.
type FileUpload struct {
	FieldName   string // form field name (e.g. "file", "audio", "voice")
	FilePath    string // path to the file on disk
	ContentType string // MIME type; empty uses application/octet-stream
}

// PostMultipart builds a multipart form with a file attachment and text fields,
// then POSTs it using the shared Client. Fields are written before the file.
func PostMultipart(url string, upload FileUpload, fields [][2]string) (*http.Response, error) {
	f, err := os.Open(upload.FilePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", upload.FilePath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, kv := range fields {
		if err := w.WriteField(kv[0], kv[1]); err != nil {
			return nil, fmt.Errorf("write field %s: %w", kv[0], err)
		}
	}

	var part io.Writer
	if upload.ContentType != "" {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`, upload.FieldName, filepath.Base(upload.FilePath)))
		h.Set("Content-Type", upload.ContentType)
		part, err = w.CreatePart(h)
	} else {
		part, err = w.CreateFormFile(upload.FieldName, filepath.Base(upload.FilePath))
	}
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy file data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart: %w", err)
	}

	return Client.Post(url, w.FormDataContentType(), &buf)
}

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
