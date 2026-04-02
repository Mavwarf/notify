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

func TestDataDirUsesHomeConfig(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := DataDir()
	want := filepath.Join(home, ".config", AppDirName)
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}
