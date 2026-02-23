# History

## Features

- Per-profile credential overrides — different profiles can target different channels *(Feb 23)*
- Notification groups — comma-separated actions in a single call *(Feb 23)*
- Direct send (`notify send`) — one-off notifications without a profile *(Feb 23)*
- Profile aliases — shorthand names for profiles *(Feb 22)*
- Template variables: `{time}`, `{Time}`, `{date}`, `{Date}`, `{hostname}` *(Feb 22)*
- Config validate command (`notify config validate`) *(Feb 22)*
- History export (`notify history export`) — JSON export of log entries *(Feb 22)*
- History summary (`notify history summary`) — aggregated usage stats per day *(Feb 22)*
- Profile inheritance (`"extends"`) — inherit actions from a parent profile *(Feb 22)*
- Exit code mapping for `notify run` — map specific codes to custom actions *(Feb 22)*
- Custom sound files — use your own WAV files in `sound` steps *(Feb 22)*
- Generic webhook step for ntfy.sh, Pushover, Home Assistant, IFTTT, etc. *(Feb 22)*
- Automatic retry for remote steps (discord, slack, telegram) *(Feb 22)*
- Environment variables in credentials (`$VAR` / `${VAR}`) *(Feb 22)*
- Sound preview (`notify play`) to audition built-in sounds *(Feb 22)*
- Notification history (`notify history`) to view recent log entries *(Feb 22)*
- Echo option (`--echo`) to print execution summary *(Feb 22)*
- Silent mode (`notify silent`) for temporary suppression *(Feb 22)*
- Remote notifications: Discord, Slack, Telegram, voice messages, voice bubbles *(Feb 20–22)*
- Cooldown / rate limiting per action *(Feb 21)*
- Config validation with multi-error reporting *(Feb 21)*
- Command wrapper (`notify run`) with exit code and duration templates *(Feb 20)*
- Quiet hours — suppress steps outside a time window *(Feb 20)*
- AFK detection — different steps when present vs away *(Feb 20)*
- Template variables: `{profile}`, `{command}`, `{duration}` *(Feb 20)*
- Opt-in event logging *(Feb 20)*
- Multi-step notification pipelines: sound, speech, toast *(Feb 19)*

---

## 2026-02-23

### Per-Profile Credential Overrides
Credentials (`discord_webhook`, `slack_webhook`, `telegram_token`,
`telegram_chat_id`) can now be set per-profile in addition to globally.
Profile credentials override global ones field-by-field — set only the
fields you want to change, the rest fall through to global. This lets
different profiles post to different Discord channels, Slack channels,
or Telegram chats. Profile credentials support the same `$VAR` / `${VAR}`
environment variable expansion as global credentials. Profile inheritance
(`"extends"`) merges parent credentials into child (child wins on conflict).
Config validation uses merged credentials, so a profile with a `discord`
step only needs `discord_webhook` set somewhere — globally or on the profile.

### Notification Groups (Comma-Separated Actions)
Fire multiple actions in a single call by separating them with commas:
`notify boss done,attention` runs `done` then `attention` in sequence.
Each action gets its own resolve, cooldown check, step filtering,
execution, logging, and echo — fully independent. If one action fails
(e.g. unknown action name), the rest still run; the process exits 1 if
any failed. Works in both direct mode and `notify run` (including
`exit_codes` mappings like `"2": "done,attention"`). Single actions
work exactly as before — no breaking changes.

### Direct Send (`notify send`)
New `notify send <type> <message>` command fires a one-off notification
without needing a profile or action defined in config. Takes the step type
and message as positional args, pulls credentials from the existing config,
and executes a single step directly. Supported types: `say`, `toast`,
`discord`, `discord_voice`, `slack`, `telegram`, `telegram_audio`,
`telegram_voice`. The `--title` flag sets a custom title for toast
notifications. `sound` and `webhook` are not supported (they need a sound
name or URL+headers, not a simple message). Template variables are expanded
in the message text. Volume is resolved from `--volume` or the config default.
Supports `--log` and `--echo` flags — log entries use `send:<type>` as the
action name (e.g. `send:telegram_voice`).

## 2026-02-22

### Profile Aliases
Profiles can now define `"aliases"` — shorthand names that resolve to the
full profile. `notify mp ready` works the same as `notify myproject ready`
when `"aliases": ["mp"]` is set on the `myproject` profile. Aliases resolve
to the canonical profile name for template variables (`{profile}` expands to
`myproject`, not `mp`) and event logging. Aliases are checked after direct
profile match but before the default fallback. `notify list` shows aliases
in the output. Config validation catches duplicate aliases (two profiles
claiming the same alias) and aliases that shadow existing profile names.
`Resolve()` now returns `(string, *Action, error)` with the canonical name.

### Template Variables: `{time}`, `{Time}`, `{date}`, `{Date}`, `{hostname}`
Five new template variables available in all text fields (`say`, `toast`,
`discord`, `slack`, `telegram`, `webhook`): `{time}` expands to compact
time (`14:30`), `{Time}` to spoken time (`2:30 PM`), `{date}` to compact
date (`2026-02-22`), `{Date}` to spoken date (`February 22, 2026`), and
`{hostname}` to the machine's hostname. Same lowercase/uppercase convention
as `{duration}`/`{Duration}` — lowercase for compact display, uppercase for
TTS. A new `baseVars()` helper in `main.go` eliminates duplication between
`runAction` and `runWrapped`.

### Config Validate Command (`notify config validate`)
New `notify config validate` command checks the config file for errors
without running any notifications. On success prints `Config OK: <path>`;
on error prints the full multi-line validation error and exits 1. Reuses
the existing `Validate()` function — zero new validation code. Also adds
`config.FindPath()` to expose config path resolution independently of
loading.

### History Export (`notify history export`)
New `notify history export [days]` subcommand outputs all log entries as a
JSON array to stdout. Each entry includes `time`, `profile`, `action`, and
`kind` (execution, cooldown, silent, other). Optional `[days]` argument
filters to recent entries. Pipe to `jq` for ad-hoc analysis. Empty logs
output `[]`.

### History Summary (`notify history summary`)
New `notify history summary [days]` subcommand parses the event log and shows
per-profile/action execution counts grouped by day (descending). Default
lookback is 7 days. Cooldown-skipped and silent-skipped invocations are shown
as a "(N skipped)" annotation alongside the execution count. `cooldown=recorded`
and `silent=enabled/disabled` entries are ignored. Also adds `notify history
clear` to delete the log file. Parser lives in `internal/eventlog/parse.go`,
co-located with the writer so format changes stay coordinated.

### Profile Inheritance (`"extends"`)
Profiles can now inherit all actions from a parent profile using
`"extends": "parent"`. The child only needs to define actions it wants to
override — all other actions are inherited from the parent. Chains are
supported (A extends B extends C) and circular chains are detected at load
time with a clear error message. Inheritance is resolved eagerly after
config load (before validation and env expansion) so all downstream code
sees fully flattened profiles. `notify list` annotates child profiles with
`(extends X)`. The `Profile` type changed from a plain map to a struct with
`Extends` and `Actions` fields, with a custom `UnmarshalJSON` that keeps
the JSON format backward-compatible — existing configs without `"extends"`
work unchanged.

### Exit Code Mapping
`notify run` previously hardcoded exit 0 → `ready` and non-zero → `error`.
New `"exit_codes"` map in the `"config"` section lets users route specific
exit codes to custom actions, e.g. `"2": "warning"` or `"130": "cancelled"`.
Keys are exit code strings, values are action names. Unmapped codes still
use the default 0→ready / non-zero→error fallback. Config validation checks
that keys parse as integers and values are non-empty. Config is now loaded
before exit code resolution in `runWrapped` to make the mapping available.

### Custom Sound Files
The `sound` step now accepts a WAV file path alongside the 7 built-in sound
names. If the value isn't a recognized built-in name, it's treated as a file
path. Relative paths are resolved against the config file's directory, so
`"pling.wav"` finds the file next to `notify-config.json`. Supports PCM WAV
files with 8-bit, 16-bit, or 24-bit sample depth, mono or stereo, at any
sample rate. Files are automatically decoded and resampled to 44100 Hz stereo
16-bit for playback through the same audio pipeline as built-in sounds.
Compressed WAV formats (A-law, mu-law, ADPCM, etc.) are rejected with a
clear error. `notify play` also accepts WAV file paths for previewing.

### Generic Webhook Step
New `webhook` step type — HTTP POST to an arbitrary URL with the message as
body. Covers ntfy.sh, Pushover, Home Assistant, IFTTT, or any custom endpoint.
URL and optional headers live on the step itself (not in credentials), so one
config can target multiple endpoints. Headers support `$VAR` / `${VAR}`
expansion for secrets via `os.ExpandEnv`. Default `Content-Type` is
`text/plain`; custom headers can override it. Body uses template variable
expansion like other step types. Requires `url` and `text` fields. Runs in
parallel with automatic retry on transient errors.

### Retry for Remote Steps
Remote notification steps (`discord`, `discord_voice`, `slack`, `telegram`,
`telegram_audio`, `telegram_voice`) now automatically retry once after a
2-second delay if the network call fails. Only the HTTP send is retried —
local prep work like TTS rendering, temp file creation, and ffmpeg conversion
is not repeated. No configuration needed; the retry is always active. This
makes notifications resilient to transient network errors without adding
complexity to the config.

### Environment Variables in Credentials
Credential values (`discord_webhook`, `slack_webhook`, `telegram_token`,
`telegram_chat_id`) now support `$VAR` and `${VAR}` syntax to reference
environment variables. Expansion happens at config load time using Go's
`os.ExpandEnv`, so all downstream code sees resolved values transparently.
Undefined variables resolve to empty strings, which the existing config
validation catches as missing credentials. Literal URLs pass through
unchanged. This makes it safe to version-control configs or share them
across machines without exposing secrets.

### Sound Preview (`notify play`)
New `notify play [sound]` command lets you audition built-in sounds without
creating a config action. With no arguments, lists all 7 available sounds
with descriptions. With a sound name, plays it immediately using the CLI
`--volume` flag or full volume by default. Unknown sound names show an error
with the list of valid names.

### Notification History (`notify history`)
New `notify history [N]` command pretty-prints the last N entries from
`notify.log` (default 10). Reads the log file location via the existing
`eventlog.LogPath()` function. If the log file doesn't exist, prints a
friendly message suggesting `--log` or `"log": true`. Useful for
reviewing recent notification activity without opening the log file
manually.

### Echo Option (`--echo`, `-E`)
New `--echo` (`-E`) CLI flag and `"echo": true` config option prints a
one-line summary of the step types that ran after each invocation, e.g.
`notify: sound, say, toast`. If all steps were filtered out:
`notify: no steps ran`. Same opt-in pattern as `--log` and `--cooldown`
(config bool + CLI flag). Useful for confirming what fired without
enabling full event logging.

### `"when": "never"` Condition
New `"when": "never"` condition that always skips the step. Add it to
temporarily disable a step without removing it from config; remove it
to re-enable.

### Telegram Voice Bubbles (`telegram_voice`)
New `telegram_voice` step type generates TTS audio, converts WAV to OGG/OPUS
via `ffmpeg`, and uploads to Telegram via the Bot API `sendVoice` endpoint.
Renders as a native voice bubble in Telegram clients (unlike `telegram_audio`
which displays as an inline audio player). Uses platform-native TTS engines
(`SayToFile`) to render speech to a temp file, then `ffmpeg -c:a libopus` for
the format conversion. Requires `ffmpeg` on PATH, `credentials.telegram_token`
and `credentials.telegram_chat_id` in config. Same `when` filtering and
template variables as other Telegram steps.

### Slack Webhook Notifications (`slack`)
New `slack` step type posts messages to a Slack channel via incoming webhook.
Uses a simple JSON POST (`{"text": "..."}`) — same pattern as the existing
`discord` step. Requires `credentials.slack_webhook` in config. Supports
template variables and `when` filtering. Runs in parallel (no audio pipeline
dependency).

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
