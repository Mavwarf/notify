package voice

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Mavwarf/notify/internal/httputil"
	"github.com/Mavwarf/notify/internal/paths"
)

const (
	cacheDirName  = "voice-cache"
	indexFileName = "voice-cache.json"
)

// CacheEntry describes a single cached voice file.
type CacheEntry struct {
	Text      string    `json:"text"`
	Voice     string    `json:"voice"`
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// Cache manages pre-generated voice WAV files.
type Cache struct {
	Dir     string
	Entries map[string]CacheEntry // hash -> entry
}

// CacheDir returns the voice cache directory path.
func CacheDir() string {
	return filepath.Join(paths.DataDir(), cacheDirName)
}

// OpenCache loads or creates the voice cache index.
func OpenCache() (*Cache, error) {
	dir := CacheDir()
	c := &Cache{
		Dir:     dir,
		Entries: make(map[string]CacheEntry),
	}

	indexPath := filepath.Join(dir, indexFileName)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("reading voice cache index: %w", err)
	}

	if err := json.Unmarshal(data, &c.Entries); err != nil {
		return nil, fmt.Errorf("parsing voice cache index: %w", err)
	}
	return c, nil
}

// Lookup checks if text has a cached WAV file. Returns the file path
// and true if found, or empty string and false if not cached.
func (c *Cache) Lookup(text string) (string, bool) {
	hash := TextHash(text)
	entry, ok := c.Entries[hash]
	if !ok {
		return "", false
	}
	wavPath := filepath.Join(c.Dir, entry.Hash+".wav")
	if _, err := os.Stat(wavPath); err != nil {
		return "", false
	}
	return wavPath, true
}

// Save writes the cache index to disk.
func (c *Cache) Save() error {
	data, err := json.MarshalIndent(c.Entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling voice cache index: %w", err)
	}
	return paths.AtomicWrite(filepath.Join(c.Dir, indexFileName), data)
}

// Add writes a WAV file to the cache and updates the index.
func (c *Cache) Add(text, voice string, wavData []byte) error {
	hash := TextHash(text)
	wavPath := filepath.Join(c.Dir, hash+".wav")

	if err := os.MkdirAll(c.Dir, paths.DirPerm); err != nil {
		return fmt.Errorf("creating voice cache dir: %w", err)
	}
	if err := os.WriteFile(wavPath, wavData, paths.FilePerm); err != nil {
		return fmt.Errorf("writing voice file: %w", err)
	}

	c.Entries[hash] = CacheEntry{
		Text:      text,
		Voice:     voice,
		Hash:      hash,
		Size:      int64(len(wavData)),
		CreatedAt: time.Now(),
	}
	return c.Save()
}

// Remove deletes a single cached WAV by hash and updates the index.
func (c *Cache) Remove(hash string) error {
	wavPath := filepath.Join(c.Dir, hash+".wav")
	_ = os.Remove(wavPath)
	delete(c.Entries, hash)
	return c.Save()
}

// Clear deletes all cached WAV files and the index.
func (c *Cache) Clear() (int, error) {
	count := len(c.Entries)
	for hash := range c.Entries {
		wavPath := filepath.Join(c.Dir, hash+".wav")
		_ = os.Remove(wavPath)
	}
	c.Entries = make(map[string]CacheEntry)
	// Remove index file.
	indexPath := filepath.Join(c.Dir, indexFileName)
	_ = os.Remove(indexPath)
	return count, nil
}

// TextHash returns a truncated SHA-256 hash of the text (16 hex chars).
func TextHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", h[:8])
}

// generateRequest is the JSON body for the OpenAI TTS API.
type generateRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed,omitempty"`
}

// Generate calls the OpenAI TTS API and returns raw WAV bytes.
func Generate(apiKey, model, voice, text string, speed float64) ([]byte, error) {
	body := generateRequest{
		Model:          model,
		Input:          text,
		Voice:          voice,
		ResponseFormat: "wav",
	}
	if speed != 0 && speed != 1.0 {
		body.Speed = speed
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httputil.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet := httputil.ReadSnippet(resp.Body)
		return nil, fmt.Errorf("openai tts returned %d: %s", resp.StatusCode, snippet)
	}

	wavData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	return wavData, nil
}
