package webhook

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Mavwarf/notify/internal/httputil"
)

// Send posts body to the given URL as text/plain. Custom headers are
// applied after the default Content-Type, so callers can override it.
// Header values are expanded with os.ExpandEnv to support $VAR secrets.
func Send(url, body string, headers map[string]string) error {
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: new request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	for k, v := range headers {
		req.Header.Set(k, os.ExpandEnv(v))
	}

	resp, err := httputil.Client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: post: %w", err)
	}
	defer resp.Body.Close()

	return httputil.CheckStatus(resp, "webhook")
}
