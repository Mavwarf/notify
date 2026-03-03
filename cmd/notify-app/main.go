package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/dashboard"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

var version = "dev"

// main launches the Wails desktop app that wraps the notification dashboard
// in a native WebView2 window with a system tray icon.
func main() {
	configPath := ""
	// Port 8811 avoids collision with the CLI dashboard's default port 8080.
	const defaultPort = 8811
	port := defaultPort

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "-c":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case "--port", "-p":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "notify-app: invalid port %q\n", args[i+1])
					os.Exit(1)
				}
				port = n
				i++
			}
		}
	}

	// Single-instance detection: POST to the running instance's /api/show
	// endpoint. If it responds, that instance brings its window to the
	// foreground and this process exits. The HTTP client's default timeout
	// (~30s) is acceptable here because a running server responds instantly.
	showURL := fmt.Sprintf("http://127.0.0.1:%d/api/show", port)
	resp, err := http.Post(showURL, "", nil)
	if err == nil {
		resp.Body.Close()
		os.Exit(0)
	}

	// Load config and open event log (same pattern as CLI).
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "notify-app: %v\n", err)
		os.Exit(1)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "notify-app: %v\n", err)
		os.Exit(1)
	}
	cfgPath, _ := config.FindPath(configPath)

	geom := loadGeometry()

	app := &App{port: port, geom: geom, ready: make(chan struct{})}

	eventlog.SetRetention(cfg.Options.RetentionDays)
	eventlog.OpenDefault(cfg.Options.Storage)
	defer eventlog.Close()

	dashboard.Version = version

	// Start the existing dashboard HTTP server in the background.
	go func() {
		if err := dashboard.Serve(cfg, cfgPath, port, false, app.ShowWindow, app.MinimizeWindow, app.QuitApp, setAlwaysOnTop); err != nil {
			fmt.Fprintf(os.Stderr, "notify-app: dashboard: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait up to 3 seconds for the dashboard server to accept connections.
	// This is generous — startup is normally <100ms, but slow disks or
	// SQLite migration on first run can add delay.
	if err := waitForServer(port, 3*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "notify-app: %v\n", err)
		os.Exit(1)
	}

	go runTray(app)

	// Loader serves a minimal HTML page to bootstrap the WebView. The Wails
	// asset server can't relay SSE flushes, so OnStartup immediately redirects
	// the WebView to the real dashboard HTTP server via JavaScript, giving the
	// browser a direct connection where SSE streaming works properly.
	loader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body style="background:#1a1b26"></body></html>`))
	})

	// Window icon comes from the embedded .syso resource (green circle
	// with white "N", same as toast icon). Regenerate with:
	//   go run ./cmd/mkicon cmd/notify-app/appicon.png
	//   cd cmd/notify-app && go-winres make
	width, height := 1200, 800
	if geom != nil {
		width, height = geom.Width, geom.Height
	}

	err = wails.Run(&options.App{
		Title:     "notify dashboard",
		Width:     width,
		Height:    height,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Handler: loader,
		},
		BackgroundColour:   &options.RGBA{R: 26, G: 27, B: 38, A: 255}, // #1a1b26
		LogLevelProduction: logger.ERROR,
		OnBeforeClose:      app.beforeClose,
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:              []interface{}{app},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "notify-app: %v\n", err)
		os.Exit(1)
	}
}

// waitForServer polls the dashboard HTTP server until it responds or the
// timeout expires.
func waitForServer(port int, timeout time.Duration) error {
	addr := fmt.Sprintf("http://127.0.0.1:%d/", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("dashboard server not ready after %s", timeout)
}
