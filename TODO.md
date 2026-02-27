# TODO

## Medium Impact

### Modern Toast Notifications (Windows)

Upgrade from `NotifyIcon.BalloonTip` to the Windows 10+ `ToastNotificationManager`
XML API. This enables rich text formatting via `hint-style` on `<text>` elements
inside `<group><subgroup>` blocks — including bold (`base`), subtitle, title,
header sizes, and subtle (60% opacity) variants. Could also support hero images,
attribution text, action buttons, and progress bars.

Would require rewriting `internal/toast/toast_windows.go` to build XML templates
and use `Windows.UI.Notifications.ToastNotificationManager` via PowerShell.
Linux and macOS toast implementations would remain unchanged.

### Chained Actions (`on_success` / `on_failure`)

Actions that trigger other actions based on step outcomes. E.g.
`"on_failure": "escalate"` would run the `escalate` action if any
step in the current action fails. Enables retry and escalation
patterns without shell scripting.

## Low Impact

### `notify watch` — File Watching

PID watching is done (`notify watch --pid <PID>`). File watching
(`notify watch --file build.log`) remains a possibility — trigger when
a file is modified. Would need `fsnotify` or polling with `os.Stat`.

### Inline Sound Effects in Voice Messages

Mix TTS speech with sound effects in a single audio message using
template syntax: `"{sound:error} Warning! {sound:alarm.wav} Build failed"`.
The output would stitch segments into one audio file: predefined sound,
TTS speech, WAV file, more TTS — all concatenated and sent as a single
voice message to Telegram/Discord.

**Implementation notes:**
- Parse message into segments (sound refs vs text-to-speech spans)
- Capture TTS output to WAV buffer instead of playing directly
  (Windows SAPI → WAV, macOS `say --output-file`)
- Predefined sounds (error, success, etc.) bundled via Go `embed`
  or resolved from a user sounds directory
- Concatenate PCM/WAV segments — normalize sample rate/format first
- Bigger lift than most features; may benefit from an audio utility package

### More Remote Notification Actions

Additional step types beyond `discord`, `slack`, and `telegram`:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli + Java |

## Tech Debt / Refactoring

### Extract `runOpts` Struct

`dispatchActions` takes 9 parameters, `executeAction` takes 10. Every new
opt-in flag requires updating 7+ function signatures (`runAction`,
`runWrapped`, `runPipe`, `watchCmd`, `hookCmd`, `dispatchActions`,
`executeAction`). Group the booleans and volume into a struct:

```go
type runOpts struct {
    Volume   int
    Log      bool
    Echo     bool
    Cooldown bool
    RunMode  bool
}
```

**Files:** `cmd/notify/main.go`, `cmd/notify/commands.go`

### Extract Shared Helpers in `eventlog`

- **`splitBlocks(content)`** — `strings.Split(content, "\n\n")` + trim +
  empty skip is repeated 3 times in `parse.go` and `summary.go`.
- **`Cutoff(days)`** — days cutoff calculation (`time.Date(...)` +
  `AddDate`) is duplicated 3 times across `parse.go`, `summary.go`, and
  `history.go`.

### Validate `VoiceConfig` Fields

`provider`, `model`, `voice`, `speed`, and `min_uses` are never validated
in `Validate()`. Invalid values pass config loading and only fail at
OpenAI API runtime. Add range/enum checks.

**File:** `internal/config/config.go`

### Voice Defaults Constants

The defaults `"nova"`, `"tts-1"`, and `1.0` are hardcoded in 3 places
(`voice.go` x2, `commands.go`). Extract a `resolveVoiceDefaults(cfg)`
helper with named constants.

### Credential Field Sync

`MergeCredentials`, `expandEnvCredentials`, and `Validate` each manually
list every credential field. Adding a new credential requires updating all
three. Consider a shared field list or reflection-based approach.

**File:** `internal/config/config.go:652-701`

### Monolithic `Validate()` Function

At 150 lines, `Validate()` handles global options, aliases, match rules,
and per-step validation all in one function. Extract sub-functions
(`validateAliases`, `validateMatchRules`, `validateSteps`) for testability
and readability.

### Dashboard Polling Efficiency

5 parallel fetch requests every 2 seconds on top of SSE. `loadHistory` is
partially redundant with SSE. No `AbortController` for in-flight requests.
Consider reducing polling frequency or using SSE-driven updates.

### Missing Tests

- `internal/mqtt` — no tests (newest package)
- `internal/procwait` — no tests (platform-specific)
- `cmd/notify/commands.go` — `sendCmd`, `configCmd`, `dryRun` untested
- `internal/slack` — only 2 tests (vs 5 for discord, 9 for telegram)
