//go:build windows

package toast

import (
	"image/png"
	"os"
	"path/filepath"

	"github.com/Mavwarf/notify/internal/icon"
	"github.com/Mavwarf/notify/internal/paths"
)

const iconFileName = "icon.png"

// EnsureIcon writes a 64×64 PNG app icon to DataDir()/icon.png if it doesn't
// already exist and returns the absolute file path. The icon is a green circle
// with a white "N" letter, generated programmatically with no external deps.
func EnsureIcon() (string, error) {
	p := filepath.Join(paths.DataDir(), iconFileName)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), paths.DirPerm); err != nil {
		return "", err
	}
	img := icon.Draw(64)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return p, nil
}
