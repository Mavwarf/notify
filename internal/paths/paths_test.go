package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCooldownKey(t *testing.T) {
	tests := []struct {
		profile, action, want string
	}{
		{"default", "ready", "default/ready"},
		{"boss", "error", "boss/error"},
		{"", "ready", "/ready"},
	}
	for _, tt := range tests {
		got := CooldownKey(tt.profile, tt.action)
		if got != tt.want {
			t.Errorf("CooldownKey(%q, %q) = %q, want %q", tt.profile, tt.action, got, tt.want)
		}
	}
}

func TestDataDirUsesAPPDATA(t *testing.T) {
	orig := os.Getenv("APPDATA")
	t.Cleanup(func() { os.Setenv("APPDATA", orig) })

	os.Setenv("APPDATA", "/fake/appdata")
	got := DataDir()
	want := filepath.Join("/fake/appdata", AppDirName)
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestDataDirFallsBackWithoutAPPDATA(t *testing.T) {
	orig := os.Getenv("APPDATA")
	t.Cleanup(func() { os.Setenv("APPDATA", orig) })

	os.Unsetenv("APPDATA")
	got := DataDir()

	// Should use ~/.config/notify or temp dir — either way must end with "notify".
	if filepath.Base(got) != AppDirName {
		t.Errorf("DataDir() = %q, expected base dir %q", got, AppDirName)
	}
}
