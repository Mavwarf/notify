# notify

**Never miss a finished build again.** Whether you're at your desk or grabbing
coffee — `notify` knows and reaches you the right way: a chime when you're
present, a Discord, Slack, or Telegram ping when you're not.

A single binary, zero-dependency notification engine for the command line.
Chain sounds, speech, toast popups, Discord messages, Discord voice messages,
Slack messages, Telegram messages, Telegram audio messages, and Telegram voice
bubbles into pipelines
— all configured in one JSON file.

## What is this for?

Long-running terminal commands finish silently. `notify` gives you instant
feedback:

```bash
notify run -- make build
```

Or chain it manually for more control:

```bash
make build && notify ready || notify error
kubectl rollout status deploy/api; notify done
```

## Installation

### Pre-built binaries

Download the latest binary for your platform from
[GitHub Releases](https://github.com/Mavwarf/notify/releases).
Place the binary somewhere on your `PATH` and copy
`notify-config.example.json` as `notify-config.json` next to it.

### From source

```bash
go install github.com/Mavwarf/notify/cmd/notify@latest
```

## Design

- **Written in Go** for easy cross-compilation and single-binary distribution.
- **Config-driven** — define notification pipelines as JSON. Each action
  combines sound, speech, toast, Discord, Slack, and Telegram steps.
- **Built-in sounds** — 7 generated tones (success, error, warning, etc.)
  created programmatically as sine-wave patterns.
- **Text-to-speech** — uses OS-native TTS engines
  (Windows SAPI, macOS `say`, Linux `espeak`).
- **Toast notifications** — native desktop notifications on all platforms
  (Windows Toast API, macOS `osascript`, Linux `notify-send`).
- **Discord webhooks** — post messages to a Discord channel via webhook,
  no external dependencies (just `net/http`).
- **Discord voice messages** — generate TTS audio and upload as a WAV
  file attachment to Discord. Same TTS engines as `say` steps.
- **Slack webhooks** — post messages to a Slack channel via incoming webhook,
  no external dependencies (just `net/http`).
- **Telegram Bot API** — send messages to a Telegram chat via bot token,
  no external dependencies (just `net/http`).
- **Telegram audio messages** — generate TTS audio and upload as a WAV
  file to Telegram via `sendAudio`. Same TTS engines as `say` steps.
- **Telegram voice bubbles** — generate TTS audio, convert WAV to OGG/OPUS
  via `ffmpeg`, and upload to Telegram via `sendVoice`. Renders as a native
  voice bubble in Telegram clients. Requires `ffmpeg` on PATH.
- **AFK detection** — conditionally run steps based on whether the user is
  at their desk or away. Play a sound when present, send a Discord, Slack,
  or Telegram message when AFK.
- **Quiet hours** — time-based `"hours:X-Y"` condition suppresses loud steps
  at night and routes to silent channels instead.
- **Cross-platform** — uses [oto](https://github.com/ebitengine/oto) for
  native audio output on Windows (WASAPI), macOS (Core Audio), and
  Linux (ALSA).

### Architecture

```
cmd/
  notify/
    main.go              CLI entry point, flag parsing, AFK wiring
    notify-config.example.json  Example config file
internal/
  audio/
    sounds.go            Generated sound definitions and PCM synthesis
    player.go            Playback engine (generated tones)
  config/
    config.go            Config loading, validation, and profile/action resolution
  cooldown/
    cooldown.go          Per-action rate limiting with file-based state
  silent/
    silent.go            Temporary notification suppression with file-based state
  discord/
    discord.go           Discord webhook integration (POST to channel)
  slack/
    slack.go             Slack incoming webhook integration (POST to channel)
  telegram/
    telegram.go          Telegram Bot API integration (sendMessage, sendAudio, sendVoice)
  ffmpeg/
    convert.go           WAV to OGG/OPUS conversion via ffmpeg
  paths/
    paths.go             Shared constants and platform-specific data directory
  idle/
    idle_windows.go      User idle time via GetLastInputInfo (Win32)
    idle_darwin.go       User idle time via ioreg HIDIdleTime
    idle_linux.go        User idle time via xprintidle
  runner/
    runner.go            Step executor (dispatches to audio/speech/toast/discord/discord_voice/slack/telegram/telegram_audio/telegram_voice)
  eventlog/
    eventlog.go          Append-only invocation log (notify.log)
  httputil/
    snippet.go           Shared HTTP response body snippet for error messages
  tmpl/
    tmpl.go              Template variable expansion ({profile}, {command}, etc.)
  shell/
    escape.go            PowerShell string escaping
    escape_darwin.go     AppleScript string escaping
  speech/
    say_windows.go       TTS via PowerShell System.Speech
    say_darwin.go        TTS via macOS say command
    say_linux.go         TTS via espeak-ng / espeak
  toast/
    toast_windows.go     Windows Toast Notification API
    toast_darwin.go      macOS osascript notifications
    toast_linux.go       Linux notify-send
```

## Usage

```bash
notify [options] [profile] <action>
notify run [options] [profile] -- <command...>
notify play [sound]                    # Preview a built-in sound (or list all)
notify test [profile]                  # Dry-run: show what would happen
notify history [N]                     # Show last N log entries (default 10)
notify silent [duration|off]           # Suppress notifications temporarily
notify list                            # List all profiles and actions
notify version                         # Show version and build date
notify help                            # Show help
```

### Options

| Flag               | Description                              |
|--------------------|------------------------------------------|
| `--volume`, `-v`   | Override volume, 0-100 (default: config or 100) |
| `--config`, `-c`   | Path to notify-config.json               |
| `--log`, `-L`      | Write invocation to notify.log         |
| `--echo`, `-E`     | Print summary of steps that ran        |
| `--cooldown`, `-C` | Enable per-action cooldown (rate limiting) |

### Config file

`notify` looks for `notify-config.json` in this order:

1. `--config <path>` (explicit)
2. `notify-config.json` next to the binary
3. `~/.config/notify/notify-config.json` (Linux/macOS) or `%APPDATA%\notify\notify-config.json` (Windows)

### Config format

```json
{
  "config": {
    "afk_threshold_seconds": 300,
    "default_volume": 100,
    "log": false,
    "echo": false,
    "cooldown": false,
    "cooldown_seconds": 30,
    "credentials": {
      "discord_webhook": "https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN",
      "slack_webhook": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
      "telegram_token": "YOUR_BOT_TOKEN",
      "telegram_chat_id": "YOUR_CHAT_ID"
    }
  },
  "profiles": {
    "default": {
      "ready": {
        "steps": [
          { "type": "sound", "sound": "success", "when": "hours:8-22" },
          { "type": "say", "text": "{command} finished in {Duration}", "when": "run" },
          { "type": "say", "text": "Ready!", "when": "direct" },
          { "type": "toast", "message": "Ready!", "when": "afk" },
          { "type": "toast", "message": "Ready!", "when": "hours:22-8" },
          { "type": "discord", "text": "Ready!", "when": "afk" },
          { "type": "discord_voice", "text": "Ready!", "when": "afk" },
          { "type": "slack", "text": "Ready!", "when": "afk" },
          { "type": "telegram", "text": "Ready!", "when": "afk" },
          { "type": "telegram_audio", "text": "Ready!", "when": "afk" },
          { "type": "telegram_voice", "text": "Ready!", "when": "afk" }
        ]
      }
    },
    "boss": {
      "ready": {
        "cooldown_seconds": 10,
        "steps": [
          { "type": "sound", "sound": "notification", "volume": 90 },
          { "type": "say", "text": "Boss is ready" },
          { "type": "toast", "title": "Boss", "message": "Ready to go" }
        ]
      }
    }
  }
}
```

- **Two top-level keys:** `"config"` for global options, `"profiles"` for
  notification pipelines.
- Each profile maps **action** names to `{ "steps": [...] }`.
  `"default"` is the fallback profile.
- **Step types:** `sound` (play a built-in sound), `say` (text-to-speech),
  `toast` (desktop notification), `discord` (post to Discord channel via webhook),
  `discord_voice` (TTS audio uploaded to Discord as WAV), `slack` (post to Slack
  channel via webhook), `telegram` (send to Telegram chat via bot),
  `telegram_audio` (TTS audio uploaded to Telegram as WAV),
  `telegram_voice` (TTS audio converted to OGG/OPUS and uploaded as voice bubble).
- **Volume priority:** per-step `volume` > CLI `--volume` > config
  `"default_volume"` > 100.
- Toast `title` defaults to the profile name if omitted.
- **Template variables:** use `{profile}` in `say` text, `toast` title/message,
  `discord`, `discord_voice`, `slack`, `telegram`, `telegram_audio`, or `telegram_voice` text to inject the runtime profile name, or `{Profile}` for
  title case (e.g. `boss` → `Boss`). When using `notify run`, `{command}`,
  `{duration}` (compact: `2m15s`), and `{Duration}` (spoken: `2 minutes and
  15 seconds`) are also available. Use `{Duration}` in `say` steps for
  natural speech output. This is especially useful with the default fallback —
  a single action definition can produce different messages depending on which
  profile name was passed on the CLI.
- **Event logging:** set `"log": true` to append every invocation to
  `notify.log` (or use `--log` on the CLI). Off by default.
- **Echo:** set `"echo": true` (or use `--echo`) to print a one-line
  summary of executed steps after each invocation, e.g.
  `notify: sound, say, toast`. Off by default.
- **Cooldown:** set `"cooldown": true` (or use `--cooldown`) to enable
  rate limiting. Set a global default with `"cooldown_seconds"` in `"config"`,
  or override per-action. Actions silently skip if the same profile+action
  was triggered within the cooldown window.
- `sound` and `say` steps run sequentially (shared audio pipeline).
  All other steps (`toast`, `discord`, `discord_voice`, `slack`,
  `telegram`, `telegram_audio`, `telegram_voice`) fire in parallel immediately.

### Available sounds

| Name           | Description                             |
|----------------|-----------------------------------------|
| `warning`      | Two-tone alternating warning signal     |
| `success`      | Ascending major chord chime             |
| `error`        | Low descending buzz indicating failure  |
| `info`         | Single clean informational beep         |
| `alert`        | Rapid high-pitched attention signal     |
| `notification` | Gentle two-note doorbell chime          |
| `blip`         | Ultra-short confirmation blip           |

### Credentials

Remote notification steps (`discord`, `discord_voice`, `slack`, `telegram`, `telegram_audio`, `telegram_voice`) need credentials stored in
the `"credentials"` object inside `"config"`:

```json
{
  "config": {
    "credentials": {
      "discord_webhook": "https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN",
      "slack_webhook": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
      "telegram_token": "YOUR_BOT_TOKEN",
      "telegram_chat_id": "YOUR_CHAT_ID"
    }
  },
  "profiles": { ... }
}
```

Credential values support environment variable expansion using `$VAR` or
`${VAR}` syntax. This lets you keep secrets out of the config file:

```json
{
  "config": {
    "credentials": {
      "discord_webhook": "$DISCORD_WEBHOOK",
      "slack_webhook": "${SLACK_WEBHOOK}",
      "telegram_token": "$TELEGRAM_TOKEN",
      "telegram_chat_id": "$TELEGRAM_CHAT_ID"
    }
  }
}
```

Undefined variables resolve to empty strings, which config validation
catches as missing credentials. Literal URLs (no `$`) pass through unchanged.

- **Discord webhook URL:** Server Settings → Integrations → Webhooks →
  New Webhook → Copy Webhook URL.
- **Slack webhook URL:** App settings → Incoming Webhooks → Add New Webhook
  to Workspace → select a channel → Copy Webhook URL.
- **Telegram bot token:** Message [@BotFather](https://t.me/BotFather) →
  `/newbot` → copy the token.
- **Telegram chat ID:** Message your bot, then open
  `https://api.telegram.org/bot<TOKEN>/getUpdates` and find `"chat":{"id":...}`.

### Discord notifications

The `discord` step type posts a message to a Discord channel via webhook.
Especially useful with `"when": "afk"` to reach you when you're away:

```json
{ "type": "discord", "text": "{Profile} build is ready", "when": "afk" }
```

The `text` field supports template variables (`{profile}`, `{Profile}`,
and `{command}`/`{duration}` in `run` mode).
Discord steps run in parallel (they don't block the audio pipeline).

### Discord voice messages

The `discord_voice` step type generates TTS audio and uploads it to Discord
as a WAV file attachment. The text is both spoken (rendered to audio) and
sent as a caption alongside the file:

```json
{ "type": "discord_voice", "text": "{Profile} build is ready", "when": "afk" }
```

Uses the same platform-native TTS engines as `say` steps. Requires
`discord_webhook` in `"credentials"`. Useful when you want an audible
notification on your phone via Discord without needing to read the message.

### Slack notifications

The `slack` step type posts a message to a Slack channel via incoming webhook.
Same pattern as Discord — especially useful with `"when": "afk"`:

```json
{ "type": "slack", "text": "{Profile} build is ready", "when": "afk" }
```

Requires `slack_webhook` in `"credentials"`. Slack steps run in parallel
(they don't block the audio pipeline).

### Telegram notifications

The `telegram` step type sends a message to a Telegram chat via the Bot API.
Same pattern as Discord — especially useful with `"when": "afk"`:

```json
{ "type": "telegram", "text": "{Profile} build is ready", "when": "afk" }
```

Requires `telegram_token` and `telegram_chat_id` in `"credentials"`.
Telegram steps run in parallel (they don't block the audio pipeline).

### Telegram audio messages

The `telegram_audio` step type generates TTS audio and uploads it to Telegram
as a WAV file via the `sendAudio` API. The text is both spoken (rendered to
audio) and sent as a caption alongside the file:

```json
{ "type": "telegram_audio", "text": "{Profile} build is ready", "when": "afk" }
```

Uses the same platform-native TTS engines as `say` steps. Requires
`telegram_token` and `telegram_chat_id` in `"credentials"`. Displays as an
inline audio player in Telegram (not a voice bubble — use `telegram_voice`
for that).

### Telegram voice messages

The `telegram_voice` step type generates TTS audio, converts it from WAV to
OGG/OPUS via `ffmpeg`, and uploads it to Telegram via the `sendVoice` API.
Renders as a native voice bubble in Telegram clients:

```json
{ "type": "telegram_voice", "text": "{Profile} build is ready", "when": "afk" }
```

Uses the same platform-native TTS engines as `say` steps. Requires
`telegram_token` and `telegram_chat_id` in `"credentials"`, and `ffmpeg`
installed on PATH. If `ffmpeg` is not available, the step returns an error.

### AFK detection

Steps can be conditionally filtered with a `"when"` condition.
AFK conditions use idle time (no keyboard/mouse input); invocation
conditions distinguish `notify run` from direct calls:

| `when` value   | Step runs when... |
|----------------|-------------------|
| *(omitted)*    | Always (default, backwards compatible) |
| `"present"`    | User is **active** (idle time below threshold) |
| `"afk"`        | User is **away** (idle time at or above threshold) |
| `"run"`        | Invoked via `notify run` (command wrapper) |
| `"direct"`     | Invoked directly (not via `notify run`) |
| `"never"`      | Never runs (temporarily disable a step) |
| `"hours:X-Y"`  | Current hour is within range (24h local time) |

Set the threshold (in seconds) in `"config"`. Default is 300 (5 minutes):

```json
{
  "config": { "afk_threshold_seconds": 300 },
  "profiles": {
    "default": {
      "ready": {
        "steps": [
          { "type": "sound", "sound": "success" },
          { "type": "say", "text": "Ready!", "when": "present" },
          { "type": "toast", "title": "AFK", "message": "Ready!", "when": "afk" }
        ]
      }
    }
  }
}
```

Idle detection is platform-native:
- **Windows**: `GetLastInputInfo` Win32 API
- **macOS**: `ioreg` HIDIdleTime
- **Linux**: `xprintidle` (must be installed)

If idle time cannot be determined (e.g. `xprintidle` not installed), notify
fails open and treats the user as present.

### Quiet hours

Use `"hours:X-Y"` to restrict steps to certain hours of the day (24-hour
local time). Useful for suppressing loud notifications at night:

```json
{
  "steps": [
    { "type": "sound", "sound": "success", "when": "hours:8-22" },
    { "type": "toast", "message": "Build done!", "when": "hours:22-8" }
  ]
}
```

- `hours:8-22` — runs when the hour is >= 8 and < 22
- `hours:22-8` — cross-midnight: runs when hour >= 22 **or** < 8
- Invalid specs are skipped (fail-closed) with a stderr warning

### Lookup logic

1. Try `profiles[profile][action]`
2. If not found, fall back to `profiles["default"][action]`
3. If neither exists, error

### Command wrapper (`notify run`)

Wrap any command to get automatic notifications on completion:

```bash
notify run -- make build              # default profile, "ready" or "error"
notify run boss -- cargo test         # boss profile
notify run -v 50 -- npm run build     # with volume override
```

`notify run` executes the command, measures its duration, then triggers
`ready` on exit code 0 or `error` on non-zero. The `--` separator is
required to distinguish notify options from the wrapped command.

Additional template variables are available in `run` mode:

| Variable     | Description                          | Example                        |
|--------------|--------------------------------------|--------------------------------|
| `{command}`  | The wrapped command string           | `make build`                   |
| `{duration}` | Compact elapsed time                 | `2m15s`                        |
| `{Duration}` | Spoken elapsed time (for TTS)        | `2 minutes and 15 seconds`     |

Use `{Duration}` in `say` steps for natural speech, `{duration}` in
toast/discord/slack for compact display.

Steps can be limited to `run` mode with `"when": "run"`, or excluded
from it with `"when": "direct"`:

```json
{ "type": "say", "text": "{command} finished in {Duration}", "when": "run" },
{ "type": "say", "text": "Ready!", "when": "direct" }
```

### Examples

```bash
notify ready                      # Run "ready" from the default profile
notify default ready              # Same as above (explicit default)
notify boss ready                 # Sound + speech + toast notification
notify -v 50 ready                # Run at 50% volume
notify -c myconfig.json dev done  # Use a specific config file
notify --log ready                # Log this invocation to notify.log
notify --echo ready               # Print summary: "notify: sound, say, toast"
notify --cooldown ready           # Enable cooldown for this invocation
notify run -- make build          # Wrap a command, auto ready/error
notify run boss -- cargo test     # Wrap with a specific profile
notify test                       # Dry-run default profile
notify test boss                  # Dry-run boss profile
notify silent 1h                  # Suppress all notifications for 1 hour
notify history                    # Show last 10 log entries
notify history 5                  # Show last 5 log entries
notify play                       # List all built-in sounds
notify play success               # Preview the success sound
notify -v 50 play blip            # Preview at 50% volume
notify silent                     # Show current silent status
notify silent off                 # Disable silent mode
```

### Event log

Event logging is opt-in. Enable it with `--log` (or `-L`) on the command
line, or set `"log": true` in the config `"config"` block. When enabled,
each invocation is appended to `notify.log` in the notify data directory
(`%APPDATA%\notify\` on Windows, `~/.config/notify/` on Linux/macOS).
Only steps that actually ran are logged (steps filtered out by AFK
detection are omitted). A blank line separates each invocation:

```
2026-02-20T14:30:05+01:00  profile=boss  action=ready  steps=sound,say,toast  afk=false
2026-02-20T14:30:05+01:00    step[1] sound  sound=notification
2026-02-20T14:30:05+01:00    step[2] say  text="Boss is ready"
2026-02-20T14:30:05+01:00    step[3] toast  title="Boss" message="Ready to go"

2026-02-20T14:35:12+01:00  profile=default  action=ready  steps=sound,toast  afk=true
2026-02-20T14:35:12+01:00    step[1] sound  sound=success
2026-02-20T14:35:12+01:00    step[2] toast  title="AFK" message="Ready!"

2026-02-20T14:35:15+01:00  profile=default  action=ready  cooldown=skipped (30s)

2026-02-20T14:40:00+01:00  silent=enabled (1h0m0s)

2026-02-20T14:40:05+01:00  profile=default  action=ready  silent=skipped

2026-02-20T14:45:00+01:00  silent=disabled
```

Template variables (`{profile}`, `{Profile}`, `{command}`, `{duration}`, etc.)
are expanded in the log so you see the actual text that was spoken or
displayed. Logging is best-effort — errors are printed to stderr but never
fail the command.

### Cooldown / rate limiting

Watch loops (`nodemon`, `cargo watch`, `fswatch`) can trigger dozens of
rebuilds per minute. Without cooldown, each rebuild fires a notification.
Cooldown silently skips duplicate notifications within a configurable window.

**Cooldown is opt-in** (off by default). Enable it with `--cooldown` (or `-C`)
on the command line, or set `"cooldown": true` in the config `"config"` block.

Set a global default duration in `"config"`, and optionally override per-action:

```json
{
  "config": { "cooldown": true, "cooldown_seconds": 30 },
  "profiles": {
    "default": {
      "ready": {
        "steps": [
          { "type": "sound", "sound": "success" },
          { "type": "say", "text": "Ready!" }
        ]
      },
      "error": {
        "cooldown_seconds": 10,
        "steps": [
          { "type": "sound", "sound": "error" }
        ]
      }
    }
  }
}
```

**Duration priority:** per-action `cooldown_seconds` > config `cooldown_seconds`.
In the example above, `ready` uses the global 30s default while `error` overrides
to 10s. If the same `profile/action` was triggered within the cooldown window,
the invocation exits immediately — no sound, no speech, no toast.
Cooldown state is stored in `%APPDATA%\notify\cooldown.json` (Windows)
or `~/.config/notify/cooldown.json` (Linux/macOS). Missing or corrupt
state files are treated as "not on cooldown" (fail-open).

### Silent mode

Sometimes you want to temporarily suppress all notifications — during a
meeting, a recording, or focused work — without editing your config.
Silent mode suppresses all notification execution for a given time window:

```bash
notify silent 1h       # Silent for 1 hour
notify silent 30m      # Silent for 30 minutes
notify silent 2h30m    # Silent for 2.5 hours
notify silent          # Show current status
notify silent off      # Disable immediately
```

During silent mode, all `notify` invocations (both direct and `notify run`)
exit immediately without firing any steps. Invocations are still logged
if event logging is enabled, so you don't lose visibility. Enabling and
disabling silent mode is also logged.

`notify test` shows silent status in its output.

Silent state is stored in `silent.json` in the notify data directory
(`%APPDATA%\notify\` on Windows, `~/.config/notify/` on Linux/macOS).
If the file is missing, corrupt, or the time has passed, notify treats
it as not silent (fail-open).

## Building

### Prerequisites

- [Go](https://go.dev/dl/) 1.24 or later
- [CMake](https://cmake.org/download/) 3.16 or later (optional — you can also
  use `go build` directly)
- [FFmpeg](https://ffmpeg.org/download.html) (optional — only needed for
  `telegram_voice` steps). Install with `winget install Gyan.FFmpeg` on Windows,
  `brew install ffmpeg` on macOS, or `apt install ffmpeg` on Linux.

### With Go directly

```bash
go build -o output/notify ./cmd/notify
```

### With CMake

```bash
cmake -B build
cmake --build build
```

The binary is placed in the `output/` directory.

### Cross-compilation (via CMake)

| Target | Platform |
|--------|----------|
| `build-notify-linux-amd64` | Linux (x86_64) |
| `build-notify-linux-arm64` | Linux (ARM64) |
| `build-notify-windows-amd64` | Windows (x86_64) |
| `build-notify-darwin-amd64` | macOS (Intel) |
| `build-notify-darwin-arm64` | macOS (Apple Silicon) |

```bash
cmake -B build
cmake --build build --target build-notify-darwin-arm64
```

Build all platforms:

```bash
cmake --build build --target build-notify-all
```

### Install

```bash
cmake -B build
cmake --install build --prefix /usr/local
```

## Platform notes

| Feature | Windows | macOS | Linux |
|---------|---------|-------|-------|
| Audio playback | WASAPI (built-in) | Core Audio (CGO) | ALSA (`libasound2-dev`) |
| Text-to-speech | System.Speech (built-in) | `say` (built-in) | `espeak-ng` or `espeak` |
| Toast notifications | Toast API (Win 10+) | `osascript` (built-in) | `notify-send` (`libnotify`) |
| Discord webhook | `net/http` (built-in) | `net/http` (built-in) | `net/http` (built-in) |
| Discord voice | TTS + `net/http` | TTS + `net/http` | TTS + `net/http` |
| Slack webhook | `net/http` (built-in) | `net/http` (built-in) | `net/http` (built-in) |
| Telegram Bot API | `net/http` (built-in) | `net/http` (built-in) | `net/http` (built-in) |
| Telegram audio | TTS + `net/http` | TTS + `net/http` | TTS + `net/http` |
| Telegram voice | TTS + `ffmpeg` + `net/http` | TTS + `ffmpeg` + `net/http` | TTS + `ffmpeg` + `net/http` |

> **Note:** Development and testing has been done primarily on Windows.
> macOS and Linux support is implemented but has not been extensively
> tested yet. If you run into issues on these platforms, please
> [open an issue](https://github.com/Mavwarf/notify/issues).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and
guidelines.

## License

MIT
