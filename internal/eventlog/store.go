package eventlog

import (
	"time"

	"github.com/Mavwarf/notify/internal/config"
	"github.com/Mavwarf/notify/internal/tmpl"
)

// Store abstracts event log storage. Implementations: FileStore (flat log file)
// and SQLiteStore (indexed SQL queries). Selected via config "storage" option.
type Store interface {
	// Write — returns error for correctness; FileStore prints to stderr (best-effort).
	Log(action string, steps []config.Step, afk bool, vars tmpl.Vars, desktop *int) error
	LogCooldown(profile, action string, seconds int) error
	LogCooldownRecord(profile, action string, seconds int) error
	LogSilent(profile, action string) error
	LogSilentEnable(d time.Duration) error
	LogSilentDisable() error

	// Read
	Entries(days int) ([]Entry, error)             // parsed entries, 0 = all
	EntriesSince(cutoff time.Time) ([]Entry, error) // entries after cutoff
	VoiceLines(days int) ([]VoiceLine, error)       // TTS text frequency
	ReadContent() (string, error)                   // raw log text

	// Maintenance
	Clean(days int) (int, error)            // remove old entries, return removed count
	RemoveProfile(name string) (int, error) // remove profile entries, return removed count
	Clear() error                           // delete all data

	// Metadata
	Path() string
}
