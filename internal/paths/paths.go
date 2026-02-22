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
	DirPerm          = 0755
	FilePerm         = 0644
)

// CooldownKey returns the map key for a profile/action pair.
func CooldownKey(profile, action string) string {
	return profile + "/" + action
}

// DataDir returns the platform-specific data directory for notify:
//   - Windows: %APPDATA%\notify
//   - Unix:    ~/.config/notify
//
// Falls back to os.TempDir()/notify if neither is available.
func DataDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, AppDirName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), AppDirName)
	}
	return filepath.Join(home, ".config", AppDirName)
}
