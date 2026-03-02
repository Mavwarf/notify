package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"os"
	"runtime"

	"github.com/energye/systray"
)

//go:embed appicon.png
var trayIcon []byte

// runTray starts the system tray icon. Must be called in a goroutine —
// systray.Run blocks until Quit is called.
func runTray(app *App) {
	// Lock this goroutine to an OS thread. On Windows, systray creates a
	// hidden window and runs a GetMessage loop — both must execute on the
	// same OS thread or messages will be lost.
	runtime.LockOSThread()
	systray.Run(func() { onTrayReady(app) }, func() {})
}

// pngToICO wraps raw PNG bytes in a minimal ICO container. Windows
// LoadImage(IMAGE_ICON) rejects raw PNG files — they must be wrapped in an
// ICO header. Since Vista, ICO supports embedded PNG data directly, so the
// image bytes are included verbatim without re-encoding.
func pngToICO(png []byte) []byte {
	buf := new(bytes.Buffer)
	// ICONDIR header
	binary.Write(buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(buf, binary.LittleEndian, uint16(1)) // type: 1 = ICO
	binary.Write(buf, binary.LittleEndian, uint16(1)) // count: 1 image

	// ICONDIRENTRY
	buf.WriteByte(0)  // width (0 = 256)
	buf.WriteByte(0)  // height (0 = 256)
	buf.WriteByte(0)  // color count
	buf.WriteByte(0)  // reserved
	binary.Write(buf, binary.LittleEndian, uint16(1))        // color planes
	binary.Write(buf, binary.LittleEndian, uint16(32))       // bits per pixel
	binary.Write(buf, binary.LittleEndian, uint32(len(png))) // image data size
	binary.Write(buf, binary.LittleEndian, uint32(6+1*16))   // offset to image data (header + 1 entry)

	// PNG data
	buf.Write(png)
	return buf.Bytes()
}

// onTrayReady configures the system tray icon, tooltip, and menu items.
// Single-click opens the menu; double-click shows the dashboard window.
func onTrayReady(app *App) {
	systray.SetIcon(pngToICO(trayIcon))
	systray.SetTooltip("notify")
	systray.SetOnClick(func(menu systray.IMenu) { menu.ShowMenu() })
	systray.SetOnDClick(func(menu systray.IMenu) { app.ShowWindow() })

	mDashboard := systray.AddMenuItem("Open Dashboard", "Show the notify dashboard window")
	mDashboard.Click(func() { app.ShowWindow() })

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Exit notify-app")
	mQuit.Click(func() {
		systray.Quit()
		os.Exit(0)
	})
}
