package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Mavwarf/notify/internal/paths"
)

// WindowGeometry stores the window's position and size between sessions.
type WindowGeometry struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// geometryPath returns the path to the persisted window geometry file.
func geometryPath() string {
	return filepath.Join(paths.DataDir(), "window.json")
}

// loadGeometry reads saved window geometry from disk. Returns nil if the file
// is missing, unreadable, or contains dimensions too small to be usable.
func loadGeometry() *WindowGeometry {
	data, err := os.ReadFile(geometryPath())
	if err != nil {
		return nil
	}
	var g WindowGeometry
	if err := json.Unmarshal(data, &g); err != nil {
		return nil
	}
	if g.Width < 100 || g.Height < 100 {
		return nil
	}
	return &g
}

// saveGeometry writes window geometry to disk. It skips saving when X or Y is
// below -10000, which indicates a minimized window on Windows (the OS moves
// minimized windows far off-screen).
func saveGeometry(g *WindowGeometry) {
	if g.X < -10000 || g.Y < -10000 {
		return
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(geometryPath(), data, 0644)
}
