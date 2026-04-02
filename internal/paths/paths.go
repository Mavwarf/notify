// Package paths resolves platform-specific directories for configuration and data files.
package paths

import (
	"os"
	"path/filepath"
)

const (
	AppDirName       = "notify"
	ConfigFileName   = "notify-config.json"
	CooldownFileName = "cooldown.json"
	SilentFileName   = "silent.json"
	LogFileName      = "notify.log"
	DirPerm  = 0755 // rwxr-xr-x — owner full, group/other read+execute
	FilePerm = 0644 // rw-r--r-- — owner read+write, group/other read-only
)

// CooldownKey returns the map key for a profile/action pair.
func CooldownKey(profile, action string) string {
	return profile + "/" + action
}

// AtomicWrite writes data to path via a temporary file + rename to avoid
// partial writes on crash. The parent directory is created if needed.
// Readers always see either the old or new contents, never a half-written file.
func AtomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), DirPerm); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, FilePerm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// DataDir returns the data directory for notify: ~/.config/notify
//
// Falls back to os.TempDir()/notify if the home directory cannot be determined.
func DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), AppDirName)
	}
	return filepath.Join(home, ".config", AppDirName)
}
