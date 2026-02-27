package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/dashboard"
	"github.com/Mavwarf/notify/internal/eventlog"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	configPath := ""
	port := 8811

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
				fmt.Sscanf(args[i+1], "%d", &port)
				i++
			}
		}
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

	eventlog.OpenDefault(cfg.Options.Storage)
	defer eventlog.Close()

	// Start the existing dashboard HTTP server in the background.
	go func() {
		if err := dashboard.Serve(cfg, cfgPath, port, false); err != nil {
			fmt.Fprintf(os.Stderr, "notify-app: dashboard: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for the dashboard server to become ready.
	if err := waitForServer(port, 3*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "notify-app: %v\n", err)
		os.Exit(1)
	}

	app := &App{port: port}

	// A minimal handler bootstraps the WebView with an empty page.
	// OnStartup then navigates to the real dashboard URL so the browser
	// connects directly — SSE streaming works without a proxy layer.
	loader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body style="background:#1a1b26"></body></html>`))
	})

	err = wails.Run(&options.App{
		Title:     "notify dashboard",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Handler: loader,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 27, B: 38, A: 255}, // #1a1b26
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app},
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
