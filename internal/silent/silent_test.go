package silent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsSilentActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "silent.json")

	s := state{SilentUntil: time.Now().Add(10 * time.Minute).Format(time.RFC3339)}
	data, _ := json.Marshal(s)
	os.WriteFile(path, data, 0644)

	if !isSilent(path) {
		t.Error("expected silent with future timestamp")
	}
}

func TestIsSilentExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "silent.json")

	s := state{SilentUntil: time.Now().Add(-10 * time.Minute).Format(time.RFC3339)}
	data, _ := json.Marshal(s)
	os.WriteFile(path, data, 0644)

	if isSilent(path) {
		t.Error("expected not silent with past timestamp")
	}
}

func TestIsSilentMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	if isSilent(path) {
		t.Error("expected not silent with missing file")
	}
}

func TestIsSilentCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "silent.json")
	os.WriteFile(path, []byte("not json"), 0644)

	if isSilent(path) {
		t.Error("expected not silent with corrupt file")
	}
}

func TestEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "silent.json")

	enable(path, 5*time.Minute)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	parsed, err := time.Parse(time.RFC3339, s.SilentUntil)
	if err != nil {
		t.Fatalf("invalid timestamp: %v", err)
	}

	// Should be roughly 5 minutes from now.
	diff := time.Until(parsed)
	if diff < 4*time.Minute || diff > 6*time.Minute {
		t.Errorf("expected ~5m from now, got %v", diff)
	}

	if !isSilent(path) {
		t.Error("expected silent after enable")
	}
}

func TestDisable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "silent.json")

	// Write a future timestamp.
	s := state{SilentUntil: time.Now().Add(10 * time.Minute).Format(time.RFC3339)}
	data, _ := json.Marshal(s)
	os.WriteFile(path, data, 0644)

	if !isSilent(path) {
		t.Fatal("precondition: expected silent before disable")
	}

	disable(path)

	if isSilent(path) {
		t.Error("expected not silent after disable")
	}
}
