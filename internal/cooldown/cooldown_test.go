package cooldown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckNoCooldownConfigured(t *testing.T) {
	// cooldownSeconds=0 means cooldown is not configured on the action.
	// Should never be on cooldown regardless of state.
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")

	// Write a recent timestamp.
	state := map[string]string{"test/ready": time.Now().Format(time.RFC3339)}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)

	if check(path, "test", "ready", 0) {
		t.Error("expected not on cooldown when cooldownSeconds=0")
	}
}

func TestCheckMissingStateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	if check(path, "test", "ready", 30) {
		t.Error("expected not on cooldown with missing state file")
	}
}

func TestCheckWithinWindow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")

	state := map[string]string{"test/ready": time.Now().Format(time.RFC3339)}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)

	if !check(path, "test", "ready", 30) {
		t.Error("expected on cooldown within window")
	}
}

func TestCheckDifferentKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")

	state := map[string]string{"test/ready": time.Now().Format(time.RFC3339)}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)

	if check(path, "test", "error", 30) {
		t.Error("expected not on cooldown for different action")
	}
}

func TestCheckExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")

	past := time.Now().Add(-60 * time.Second).Format(time.RFC3339)
	state := map[string]string{"test/ready": past}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)

	if check(path, "test", "ready", 30) {
		t.Error("expected not on cooldown after expiry")
	}
}

func TestCheckCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")
	os.WriteFile(path, []byte("not json"), 0644)

	if check(path, "test", "ready", 30) {
		t.Error("expected not on cooldown with corrupt state file")
	}
}

func TestRecordPrunesExpiredEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")

	fresh := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	stale := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	state := map[string]string{
		"proj/fresh": fresh,
		"proj/stale": stale,
	}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)

	record(path, "proj", "new")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := got["proj/fresh"]; !ok {
		t.Error("expected fresh entry to survive pruning")
	}
	if _, ok := got["proj/new"]; !ok {
		t.Error("expected new entry to be present")
	}
	if _, ok := got["proj/stale"]; ok {
		t.Error("expected stale entry to be pruned")
	}
}

func TestRecordPrunesCorruptEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cooldown.json")

	state := map[string]string{
		"proj/corrupt": "not-a-timestamp",
	}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)

	record(path, "proj", "new")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := got["proj/corrupt"]; ok {
		t.Error("expected corrupt entry to be pruned")
	}
	if _, ok := got["proj/new"]; !ok {
		t.Error("expected new entry to be present")
	}
}

func TestRecordCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "cooldown.json")

	record(path, "test", "ready")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	var state map[string]string
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	ts, ok := state["test/ready"]
	if !ok {
		t.Fatal("key test/ready not found in state")
	}

	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("invalid timestamp: %v", err)
	}

	if time.Since(parsed) > 5*time.Second {
		t.Errorf("timestamp too old: %v", parsed)
	}
}
