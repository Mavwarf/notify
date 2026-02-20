# History

## 2026-02-20

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
Added append-only invocation log at `~/.notify.log` with summary and
per-step detail lines. Template variables are expanded in log output.
Best-effort — errors print to stderr but never fail the command.

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
