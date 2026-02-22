# History

## Features

- Silent mode (`notify silent`) for temporary suppression *(Feb 22)*
- Remote notifications: Discord, Telegram, voice messages *(Feb 20–22)*
- Cooldown / rate limiting per action *(Feb 21)*
- Config validation with multi-error reporting *(Feb 21)*
- Command wrapper (`notify run`) with exit code and duration templates *(Feb 20)*
- Quiet hours — suppress steps outside a time window *(Feb 20)*
- AFK detection — different steps when present vs away *(Feb 20)*
- Template variables: `{profile}`, `{command}`, `{duration}` *(Feb 20)*
- Opt-in event logging *(Feb 20)*
- Multi-step notification pipelines: sound, speech, toast *(Feb 19)*

---

## 2026-02-22

### Cooldown Auto-Pruning
`record()` now prunes expired entries from `cooldown.json` on every write.
Entries older than 24 hours or with unparseable timestamps are deleted
before the new entry is written, preventing unbounded file growth from
long-running watch loops.

### Silent Mode (`notify silent`)
New `notify silent <duration>` command temporarily suppresses all
notification execution without editing the config. Supports Go-style
duration strings (`30s`, `5m`, `1h`, `2h30m`). `notify silent off`
disables immediately; `notify silent` (no args) shows current status.
During silent mode, both direct invocations and `notify run` exit
immediately — no sound, no speech, no toast, no remote notifications.
Suppressed invocations are still logged when event logging is enabled,
as are enable/disable state changes.
`notify test` shows silent status in its output. State is stored in
`silent.json` in the notify data directory, with fail-open semantics
(missing, corrupt, or expired files are treated as not silent).

### Telegram Audio Messages (`telegram_audio`)
New `telegram_audio` step type generates TTS audio and uploads it to Telegram
as a WAV file via the Bot API `sendAudio` endpoint. Uses platform-native TTS
engines (`SayToFile`) to render speech to a temp file, then uploads via
multipart POST with the text as a caption. Same `when` filtering and template
variables as the existing `telegram` step. Requires `credentials.telegram_token`
and `credentials.telegram_chat_id` in config. Displays as an inline audio
player in Telegram.

### Toast Validation
Config validation now checks that `toast` steps have a `message` field set.
Previously only `sound`, `say`, `discord`, and `telegram` steps had
required-field validation.

### API Error Body in Messages
Discord and Telegram API error messages now include up to 200 bytes of the
response body alongside the HTTP status code. Previously only the status
code was reported, making it hard to debug issues like invalid tokens or
rate limiting.

### Test Coverage
Added unit tests for `audio` (PCM generation, volume scaling, sound
registry completeness) and `eventlog` (step detail formatting for all
step types, template expansion, default toast title). Added missing
config validation tests for `discord_voice` and `telegram_audio`
credential and required-field checks, plus `log` config parsing tests.

### AppleScript Escaping Extraction
Moved inline `escapeAppleScript()` from `toast_darwin.go` to
`shell/escape_darwin.go` as `EscapeAppleScript()`, consistent with the
existing `EscapePowerShell()` pattern in `shell/escape.go`.

### Runner Credential Check Cleanup
Removed redundant credential validation from `runner.go` — config
validation already catches missing credentials at load time. The runner
checks were defensive duplicates that could never trigger after a
successful `Validate()` call.

### Help Output
`notify help` and `notify -h` now show a link to the GitHub documentation
at the top of the output.

## 2026-02-21

### Discord Voice Messages (`discord_voice`)
New `discord_voice` step type generates TTS audio and uploads it to Discord
as a WAV file attachment. Uses platform-native TTS engines (`SayToFile`)
to render speech to a temp file, then uploads via multipart POST with the
text as a caption. Same `when` filtering and template variables as the
existing `discord` step. Requires `credentials.discord_webhook` in config.

### Dry-Run Command (`notify test`)
New `notify test [profile]` command loads config, validates it, detects
AFK state, then shows what would happen for every action in the profile
without actually firing notifications. Each step is marked RUN or SKIP
based on current `when` conditions. Useful for debugging new configs.

### Credential Completeness Validation
Config validation now checks that `discord` steps have
`credentials.discord_webhook` set, and `telegram` steps have both
`credentials.telegram_token` and `credentials.telegram_chat_id` set.
Catches missing credentials at config load time instead of at execution.

### `detectAFK` Testability
The idle detection function is now a package-level variable, allowing
unit tests to inject mock idle times. Three new tests cover AFK, present,
and error (fail-open) scenarios.

### Telegram Notifications
New `telegram` step type sends messages via the Telegram Bot API.
Requires `telegram_token` and `telegram_chat_id` in the config
credentials block. Uses form-encoded POST to the `sendMessage` endpoint.
Same template variables and `when` filtering as the existing `discord`
step type. The `Execute` function now takes a `Credentials` struct
instead of a single webhook URL, making it straightforward to add
future remote notification platforms.

### Config Validation
New `Validate()` function in the config package catches common mistakes
at load time instead of failing silently at runtime. Checks: unknown
step types, invalid `when` conditions (including malformed `hours:X-Y`
specs), missing required fields per step type (`sound` for sound steps,
`text` for say/discord), volume ranges (0-100) on both per-step and
global default, negative cooldown/AFK thresholds, and empty actions.
Collects all problems into a single multi-line error. Called
automatically after config load in all code paths.

### Path Resolution Consolidation
Extracted shared `internal/paths` package consolidating triplicated
platform-specific path resolution logic from config, cooldown, and
eventlog into a single `DataDir()` function. Shared constants for file
names and permissions replace scattered magic numbers. Extracted
`resolveVolume`, `resolveCooldown`, `detectAFK`, and `shouldLog` helpers
in `main.go` to eliminate duplication between `runAction` and `runWrapped`.

### Cooldown / Rate Limiting
Per-action cooldown prevents notification spam from watch loops and file
watchers. Cooldown is opt-in: enable with `--cooldown` (`-C`) on the CLI
or `"cooldown": true` in the config `"config"` block. Set a global default
duration with `"cooldown_seconds"` in config, or override per-action.
When enabled, if the same profile+action was triggered within the cooldown
window, the invocation exits silently. Cooldown state is stored in
`cooldown.json` in the notify data directory. Skipped and recorded cooldown
events are logged when event logging is enabled.

### Log File Location
The event log (`notify.log`) and cooldown state (`cooldown.json`) now
live in the notify data directory (`%APPDATA%\notify\` on Windows,
`~/.config/notify/` on Linux/macOS) instead of the home directory.

## 2026-02-20

### Opt-in Event Logging
Event logging is now opt-in instead of always writing a log on every
invocation. Enable with `--log` (`-L`) on the CLI or `"log": true` in
the config `"config"` block. Without either, no log file is written.

### Quiet Hours (`hours:X-Y`)
New time-based `"when"` condition suppresses steps outside a given hour
range. `"hours:8-22"` runs the step only between 8am and 10pm;
`"hours:22-8"` handles cross-midnight ranges. Invalid specs are
fail-closed (step skipped with stderr warning). Pairs naturally with
AFK detection — suppress sound at night while keeping toast/discord
active, all within one profile.

### Command Wrapper (`notify run`)
New `notify run [profile] -- <command...>` subcommand wraps an arbitrary
command, captures its exit code and duration, then automatically triggers
the `ready` action on success (exit 0) or `error` on failure. Three new
template variables: `{command}` (the wrapped command), `{duration}`
(compact: `2m15s`), and `{Duration}` (spoken: `2 minutes and 15 seconds`
— ideal for TTS). The `--` separator is required to distinguish notify
flags from the wrapped command. The wrapped command's exit code is
preserved as notify's own exit code.

New `"when"` conditions `"run"` and `"direct"` let steps target a
specific invocation mode — use `"when": "run"` for steps that only
make sense with command context (like speaking the duration), and
`"when": "direct"` for steps that should only run in normal invocations.

### CI and Collaboration
Added CI workflow that runs `go vet`, tests, and cross-platform build
checks on every push and PR to `main`. Added `CONTRIBUTING.md` with
setup instructions and PR guidelines. Added issue templates for bug
reports and feature requests.

### Version Command
`notify version` (also `-V`, `--version`) prints version, build date,
and platform. Version and build date are injected via `-ldflags` in CI;
local builds show `dev`. Help output now shows the version in its
header and author credit with links at the bottom.

### GitHub Actions Release Workflow
Added automated release pipeline triggered by version tags (`v*`).
Builds binaries for all 5 platform targets (linux/amd64, linux/arm64,
windows/amd64, darwin/amd64, darwin/arm64), runs tests first, then
creates a GitHub Release with all binaries, `THIRD_PARTY_LICENSES`,
and the example config attached. Linux builds use `CGO_ENABLED=1`
with ALSA headers (oto requires CGO on Linux); Windows and macOS
builds use `CGO_ENABLED=0` (oto uses purego on these platforms).

### Config Structure Reorganization
Config file now uses explicit `"config"` and `"profiles"` top-level
sections instead of a flat layout. Global options (AFK threshold,
default volume, credentials) live under `"config"`, profile definitions
live under `"profiles"`. This also simplified the custom UnmarshalJSON
from raw-map plucking to a standard type-alias approach with defaults.

### Default Volume in Config
New `"default_volume"` key in the `"config"` section (0-100) sets the
baseline volume when `--volume` is not passed on the command line.
Per-step `volume` overrides still take highest priority. Defaults to
100 if omitted.

### Discord Webhook Notifications
New `discord` step type posts messages to a Discord channel via webhook.
Credentials are stored in `"config"` → `"credentials"`. Discord steps
run in parallel (no audio pipeline dependency) and
support template variables (`{profile}`, `{Profile}`). Pairs naturally
with AFK detection — use `"when": "afk"` to send a Discord message when
you're away from your machine.

### AFK Detection
Per-step `"when"` condition flag (`"afk"`, `"present"`, or omitted for
always-run) lets a single action handle both cases — e.g. play a sound
when present, show a toast when away. Configurable idle threshold via
`"afk_threshold_seconds"` in notify-config.json (default 300s).
Platform-native idle detection: Windows `GetLastInputInfo`, macOS
`ioreg` HIDIdleTime, Linux `xprintidle`. Fails open on error.

### Event Log Improvements
Log now records only steps that actually executed (AFK-filtered steps
are omitted). Blank line separates each invocation for readability.

### Code Quality
Extracted duplicated `expand`/`titleCase` helpers into `internal/tmpl`
and `escapePowerShell` into `internal/shell`. Added unit tests for
config parsing, template expansion, PowerShell escaping, and AFK step
filtering.

### Template Variables
Added `{profile}` and `{Profile}` (title-cased) placeholders for use
in `say` text, `toast` title/message, and `discord` text. Useful with the default
profile fallback — one action definition produces different messages
depending on which profile was invoked.

### Config File Rename
Renamed config from generic `config.json` to `notify-config.json` to
avoid conflicts with other tools.

### Event Log
Added append-only invocation log (`notify.log` in the notify data
directory) with summary and per-step detail lines. Template variables
are expanded in log output. Best-effort — errors print to stderr but
never fail the command.

### Default Profile
Profile argument is now optional; omitting it defaults to `"default"`.

## 2026-02-19

### Initial Release
`notify` CLI tool that runs multi-step notification pipelines from a
JSON config file. Each action can combine sound, speech, and toast
steps. Supports multiple profiles with automatic fallback to `"default"`
profile. Config file resolution: explicit `--config` path, next to
binary, or user config directory. Built with Go and
[oto](https://github.com/ebitengine/oto) for cross-platform audio
(Windows WASAPI, macOS Core Audio, Linux ALSA). CMake build system with
cross-compilation targets. Volume control via `--volume` flag (0-100).
