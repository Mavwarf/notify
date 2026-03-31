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
	geom  *WindowGeometry // restored geometry from previous session, nil if none
	ready chan struct{}    // closed when Wails startup completes
}

// startup is called by Wails once the WebView window is ready. It stores the
// context for later window operations and signals readiness to any goroutine
// blocked on the ready channel (e.g. tray callbacks calling ShowWindow).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Restore saved window position from previous session.
	if a.geom != nil {
		wailsRuntime.WindowSetPosition(ctx, a.geom.X, a.geom.Y)
	}
	// Wails v2 has no Navigate API, so we use WindowExecJS to set
	// window.location, redirecting the WebView from the loader page to
	// the live dashboard HTTP server.
	url := fmt.Sprintf("http://127.0.0.1:%d", a.port)
	wailsRuntime.WindowExecJS(ctx, fmt.Sprintf("window.location.href = '%s';", url))
	close(a.ready)
}

// shutdown is called by Wails when the window closes. Intentionally a no-op;
// actual cleanup happens via systray.Quit or os.Exit in beforeClose/QuitApp.
func (a *App) shutdown(ctx context.Context) {}

// saveWindowGeometry captures the current window position and size and persists
// them to disk for restoration on next launch.
func (a *App) saveWindowGeometry() {
	x, y := wailsRuntime.WindowGetPosition(a.ctx)
	w, h := wailsRuntime.WindowGetSize(a.ctx)
	saveGeometry(&WindowGeometry{X: x, Y: y, Width: w, Height: h})
}

// beforeClose intercepts the window close event and exits the application.
func (a *App) beforeClose(ctx context.Context) bool {
	a.saveWindowGeometry()
	systray.Quit()
	os.Exit(0)
	return false
}

// ShowWindow brings the dashboard window to the foreground. It blocks on the
// ready channel to prevent a nil-context panic if called before Wails startup
// completes (e.g. from a tray callback or /api/show during early init).
func (a *App) ShowWindow() {
	<-a.ready
	wailsRuntime.WindowShow(a.ctx)
}

// MinimizeWindow hides the dashboard window to the system tray.
func (a *App) MinimizeWindow() {
	<-a.ready
	wailsRuntime.WindowHide(a.ctx)
}

// QuitApp fully exits the application by tearing down the system tray and
// terminating the process. Called from the dashboard's Quit button.
func (a *App) QuitApp() {
	a.saveWindowGeometry()
	systray.Quit()
	os.Exit(0)
}
