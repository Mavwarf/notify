package voice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTextHash(t *testing.T) {
	// Deterministic: same input always produces same output.
	h1 := TextHash("hello world")
	h2 := TextHash("hello world")
	if h1 != h2 {
		t.Errorf("TextHash not deterministic: %q != %q", h1, h2)
	}

	// Length: 16 hex chars.
	if len(h1) != 16 {
		t.Errorf("TextHash length = %d, want 16", len(h1))
	}

	// Different input produces different hash.
	h3 := TextHash("different text")
	if h1 == h3 {
		t.Errorf("TextHash collision: %q and %q both produce %q", "hello world", "different text", h1)
	}

	// Empty string has a hash too.
	h4 := TextHash("")
	if len(h4) != 16 {
		t.Errorf("TextHash empty string length = %d, want 16", len(h4))
	}
}

func TestCacheCRUD(t *testing.T) {
	// Use a temp directory for the cache.
	dir := t.TempDir()

	c := &Cache{
		Dir:     dir,
		Entries: make(map[string]CacheEntry),
	}

	// Initially empty.
	if _, ok := c.Lookup("hello"); ok {
		t.Error("Lookup should return false for empty cache")
	}

	// Add an entry.
	wavData := []byte("RIFF....fake wav data")
	if err := c.Add("hello", "nova", wavData); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Lookup should find it.
	path, ok := c.Lookup("hello")
	if !ok {
		t.Fatal("Lookup should return true after Add")
	}
	hash := TextHash("hello")
	wantPath := filepath.Join(dir, hash+".wav")
	if path != wantPath {
		t.Errorf("Lookup path = %q, want %q", path, wantPath)
	}

	// WAV file should exist on disk.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != string(wavData) {
		t.Error("WAV data mismatch")
	}

	// Index file should exist.
	indexPath := filepath.Join(dir, indexFileName)
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("Index file not found: %v", err)
	}

	// Entry should be in the map.
	entry, ok := c.Entries[hash]
	if !ok {
		t.Fatal("Entry not found in map")
	}
	if entry.Text != "hello" {
		t.Errorf("Entry.Text = %q, want %q", entry.Text, "hello")
	}
	if entry.Voice != "nova" {
		t.Errorf("Entry.Voice = %q, want %q", entry.Voice, "nova")
	}
	if entry.Size != int64(len(wavData)) {
		t.Errorf("Entry.Size = %d, want %d", entry.Size, len(wavData))
	}

	// Remove the entry.
	if err := c.Remove(hash); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, ok := c.Lookup("hello"); ok {
		t.Error("Lookup should return false after Remove")
	}
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Error("WAV file should be deleted after Remove")
	}
}

func TestCacheClear(t *testing.T) {
	dir := t.TempDir()

	c := &Cache{
		Dir:     dir,
		Entries: make(map[string]CacheEntry),
	}

	// Add two entries.
	c.Add("one", "nova", []byte("data1"))
	c.Add("two", "nova", []byte("data2"))

	if len(c.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(c.Entries))
	}

	count, err := c.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Clear returned count %d, want 2", count)
	}
	if len(c.Entries) != 0 {
		t.Error("Entries should be empty after Clear")
	}
}

func TestOpenCacheEmpty(t *testing.T) {
	// Override CacheDir by creating a cache in a temp dir.
	dir := t.TempDir()
	c := &Cache{
		Dir:     dir,
		Entries: make(map[string]CacheEntry),
	}

	// Save empty index, then reload.
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload by reading the index file.
	c2 := &Cache{
		Dir:     dir,
		Entries: make(map[string]CacheEntry),
	}
	data, err := os.ReadFile(filepath.Join(dir, indexFileName))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if err := json.Unmarshal(data, &c2.Entries); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(c2.Entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(c2.Entries))
	}
}

func TestLookupMissingFile(t *testing.T) {
	dir := t.TempDir()

	c := &Cache{
		Dir:     dir,
		Entries: make(map[string]CacheEntry),
	}

	// Add entry to index but don't write the WAV file.
	hash := TextHash("ghost")
	c.Entries[hash] = CacheEntry{
		Text: "ghost",
		Hash: hash,
	}

	// Lookup should return false because the WAV file doesn't exist.
	if _, ok := c.Lookup("ghost"); ok {
		t.Error("Lookup should return false when WAV file is missing")
	}
}
