package main

import (
	"context"
	"fmt"
	"os"

	"github.com/energye/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is a minimal struct for the Wails application lifecycle.
type App struct {
	ctx   context.Context
	port  int
	ready chan struct{} // closed when Wails startup completes
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Navigate the WebView directly to the dashboard HTTP server.
	// This bypasses the Wails asset server so SSE streaming works natively.
	url := fmt.Sprintf("http://127.0.0.1:%d", a.port)
	wailsRuntime.WindowExecJS(ctx, fmt.Sprintf("window.location.href = '%s';", url))
	close(a.ready)
}

func (a *App) shutdown(ctx context.Context) {}

// beforeClose intercepts the window close event. Shift+close exits fully;
// normal close hides to tray.
func (a *App) beforeClose(ctx context.Context) bool {
	if isShiftHeld() {
		systray.Quit()
		os.Exit(0)
		return false
	}
	wailsRuntime.WindowHide(a.ctx)
	return true // prevent close → hide to tray
}

func (a *App) ShowWindow() {
	<-a.ready // wait for Wails to be initialized
	wailsRuntime.WindowShow(a.ctx)
}
