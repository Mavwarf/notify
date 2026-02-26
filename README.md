# notify

**Never miss a finished build again.** Whether you're at your desk or grabbing
coffee — `notify` knows and reaches you the right way: a chime when you're
present, a Discord, Slack, or Telegram ping when you're not.

A single binary, zero-dependency notification engine for the command line.
Chain sounds, speech, toast popups, Discord messages, Discord voice messages,
Slack messages, Telegram messages, Telegram audio messages, Telegram voice
bubbles, and generic webhooks into pipelines
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
  combines sound, speech, toast, Discord, Slack, Telegram, and webhook steps.
- **Built-in sounds** — 7 generated tones (success, error, warning, etc.)
  created programmatically as sine-wave patterns. Also supports custom WAV files.
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
- **Generic webhooks** — HTTP POST to any URL with custom headers. Covers
  ntfy.sh, Pushover, Home Assistant, IFTTT, or any custom endpoint.
- **AFK detection** — conditionally run steps based on whether the user is
  at their desk or away. Play a sound when present, send a Discord, Slack,
  or Telegram message when AFK.
- **Quiet hours** — time-based `"hours:X-Y"` condition suppresses loud steps
  at night and routes to silent channels instead.
- **Shell hook** — `notify shell-hook install` adds a precmd/preexec hook
  to bash, zsh, or PowerShell that automatically notifies after any command
  exceeding a time threshold (default 30s). No `notify run` wrapping needed.
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
  dashboard/
    dashboard.go         Web dashboard HTTP server, API handlers, SSE
    static/index.html    Embedded frontend (HTML + inline CSS + JS)
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
  procwait/
    wait_windows.go      Wait for PID exit via OpenProcess + WaitForSingleObject
    wait_unix.go         Wait for PID exit via kill(pid, 0) polling
  webhook/
    webhook.go           Generic HTTP POST webhook integration
  voice/
    voice.go             AI voice cache management and OpenAI TTS API client
  runner/
    runner.go            Step executor (dispatches to audio/speech/toast/discord/discord_voice/slack/telegram/telegram_audio/telegram_voice/webhook)
  eventlog/
    eventlog.go          Append-only invocation log (notify.log)
  httputil/
    snippet.go           Shared HTTP response body snippet for error messages
  tmpl/
    tmpl.go              Template variable expansion ({profile}, {command}, etc.)
  shell/
    escape.go            PowerShell string escaping
    escape_darwin.go     AppleScript string escaping
    hook.go              Shell hook snippet generation, install/uninstall (bash/zsh/PowerShell)
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
notify [options] [profile] <action[,action2,...]>
notify run [options] [profile] -- <command...>
notify watch --pid <PID> [options] [profile]  # Watch a process, notify on exit
notify pipe [options] [profile] [--match <pat> <action>...]  # Stream mode
notify send [--title <title>] <type> <message>  # Send a one-off notification
notify init                            # Interactive config generator
notify init --defaults                 # Write built-in defaults to file
notify shell-hook install               # Auto-notify after long commands
notify shell-hook uninstall            # Remove shell hook
notify shell-hook status               # Check if hook is installed
notify play [sound]                    # Preview a built-in sound (or list all)
notify test [profile]                  # Dry-run: show what would happen
notify dashboard [--port N] [--open]   # Local web UI (default port 8080)
notify config validate                 # Check config file for errors
notify history [N]                     # Show last N log entries (default 10)
notify history summary [days|all]      # Show action counts per day (default 7)
notify history watch                   # Live today's summary (refreshes every 2s, x or Esc to exit)
notify history export [days]           # Export log entries as JSON (default: all)
notify history clean [days]             # Remove old entries, keep last N days
notify history clear                   # Delete the log file
notify voice generate [--min-uses N]    # Generate AI voice files for frequent say steps
notify voice play [text]               # Play all cached voices, or one matching text
notify voice list                      # List cached AI voice files
notify voice clear                     # Delete all cached voice files
notify voice stats [days|all]          # Show say step text usage frequency
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
| `--match`, `-M`    | Select action by output pattern: `--match <pattern> <action>` (repeatable, `run`/`pipe` mode) |
| `--log`, `-L`      | Write invocation to notify.log         |
| `--echo`, `-E`     | Print summary of steps that ran        |
| `--cooldown`, `-C` | Enable per-action cooldown (rate limiting) |
| `--heartbeat`, `-H` | Periodic notification during `run` (e.g. `5m`, `2m30s`) |
| `--port`, `-p`     | Port for `dashboard` command (default: 8080) |
| `--open`, `-O`     | Open dashboard in a chromeless browser window |

### Config file

`notify` looks for `notify-config.json` in this order:

1. `--config <path>` (explicit)
2. `notify-config.json` next to the binary
3. `~/.config/notify/notify-config.json` (Linux/macOS) or `%APPDATA%\notify\notify-config.json` (Windows)
4. **Built-in defaults** — if no config file exists, `notify` uses a built-in `default` profile with four actions (`ready`, `error`, `done`, `attention`) using local sound + speech. No setup needed for basic usage.

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
    "exit_codes": {
      "2": "warning",
      "130": "cancelled"
    },
    "output_lines": 0,
    "heartbeat_seconds": 0,
    "shell_hook_threshold": 30,
    "openai_voice": {
      "model": "tts-1",
      "voice": "nova",
      "speed": 1.0,
      "min_uses": 3
    },
    "credentials": {
      "discord_webhook": "https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN",
      "slack_webhook": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
      "telegram_token": "YOUR_BOT_TOKEN",
      "telegram_chat_id": "YOUR_CHAT_ID",
      "openai_api_key": "$OPENAI_API_KEY"
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
          { "type": "telegram_voice", "text": "Ready!", "when": "afk" },
          { "type": "webhook", "url": "https://ntfy.sh/mytopic", "text": "Ready!", "when": "afk" }
        ]
      }
    },
    "boss": {
      "aliases": ["b"],
      "match": { "dir": "/work/" },
      "ready": {
        "cooldown_seconds": 10,
        "steps": [
          { "type": "sound", "sound": "notification", "volume": 90 },
          { "type": "say", "text": "Boss is ready" },
          { "type": "toast", "title": "Boss", "message": "Ready to go" }
        ]
      }
    },
    "quiet": {
      "extends": "default",
      "ready": {
        "steps": [
          { "type": "sound", "sound": "blip", "volume": 30 }
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
- **Profile inheritance:** add `"extends": "parent"` to inherit all actions
  from another profile and override only specific ones. Chains are supported
  (A extends B extends C). Circular chains are detected at load time.
- **Profile aliases:** add `"aliases": ["b", "boss2"]` to create shorthand
  names for a profile. `notify b ready` resolves to the profile that
  declares `b` as an alias. Template variables like `{profile}` use the
  real profile name, not the alias. Duplicates and shadowing of real
  profile names are caught at validation time.
- **Profile auto-selection:** add a `"match"` object to a profile to
  auto-select it when the profile argument is omitted. Conditions:
  `"dir"` (substring match against the working directory, forward-slash
  normalized) and `"env"` (`KEY=VALUE` check). All conditions are AND —
  both must match. If multiple profiles match, the first alphabetically
  wins. Falls back to `"default"` when no match rule is satisfied.
  Explicit profile (`notify boss done`) always takes priority.
- **Step types:** `sound` (play a built-in sound or WAV file), `say` (text-to-speech),
  `toast` (desktop notification), `discord` (post to Discord channel via webhook),
  `discord_voice` (TTS audio uploaded to Discord as WAV), `slack` (post to Slack
  channel via webhook), `telegram` (send to Telegram chat via bot),
  `telegram_audio` (TTS audio uploaded to Telegram as WAV),
  `telegram_voice` (TTS audio converted to OGG/OPUS and uploaded as voice bubble),
  `webhook` (HTTP POST to any URL with custom headers).
- **Volume priority:** per-step `volume` > CLI `--volume` > config
  `"default_volume"` > 100.
- Toast `title` defaults to the profile name if omitted.
- **Template variables:** use `{profile}` in `say` text, `toast` title/message,
  `discord`, `discord_voice`, `slack`, `telegram`, `telegram_audio`,
  `telegram_voice`, or `webhook` text to inject the runtime profile name, or `{Profile}` for
  title case (e.g. `boss` → `Boss`). `{time}` expands to the current time
  (`14:30`), `{Time}` to a spoken form (`2:30 PM`), `{date}` to the current
  date (`2026-02-22`), `{Date}` to a spoken form (`February 22, 2026`), and
  `{hostname}` to the machine's hostname. When using `notify run`, `{command}`,
  `{duration}` (compact: `2m15s`), `{Duration}` (spoken: `2 minutes and
  15 seconds`), and `{output}` (last N lines of command output, requires
  `"output_lines"` in config) are also available. In `notify pipe` mode,
  `{output}` contains the matched line from stdin. Use `{Duration}` in `say`
  steps for natural speech output. This is especially useful with the default fallback —
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
- **Exit code mapping:** by default, `notify run` triggers `ready` on
  exit 0 and `error` on non-zero. Add `"exit_codes"` to `"config"` to
  map specific codes to different actions, e.g. `"2": "warning"`. Unmapped
  codes still use the default 0→ready / non-zero→error fallback.
- `sound` and `say` steps run sequentially (shared audio pipeline).
  All other steps (`toast`, `discord`, `discord_voice`, `slack`,
  `telegram`, `telegram_audio`, `telegram_voice`, `webhook`) fire in parallel
  immediately.

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

### Custom sound files

Set `"sound"` to a file path instead of a built-in name to play your own WAV:

```json
{ "type": "sound", "sound": "doorbell.wav" }
{ "type": "sound", "sound": "C:/sounds/doorbell.wav" }
```

Relative paths are resolved against the config file's directory, so
`"doorbell.wav"` looks for the file next to your `notify-config.json`.
Absolute paths work too.

Requirements: WAV format, PCM only (no compression). Any sample rate, bit
depth (8/16/24-bit), and channel count (mono/stereo) are supported — files
are automatically converted to 44100 Hz stereo for playback.

### Credentials

Remote notification steps (`discord`, `discord_voice`, `slack`, `telegram`,
`telegram_audio`, `telegram_voice`) need credentials stored in
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

#### Per-profile credential overrides

Profiles can override global credentials field-by-field so different
profiles post to different channels. Set only the fields you want to
change — the rest fall through to global:

```json
{
  "config": {
    "credentials": {
      "discord_webhook": "https://discord.com/api/webhooks/.../general",
      "telegram_token": "$TELEGRAM_TOKEN",
      "telegram_chat_id": "$TELEGRAM_CHAT_ID"
    }
  },
  "profiles": {
    "projectA": {
      "credentials": {
        "discord_webhook": "https://discord.com/api/webhooks/.../project-a"
      },
      "done": {
        "steps": [
          { "type": "discord", "text": "Project A done!" },
          { "type": "telegram", "text": "Project A done!" }
        ]
      }
    }
  }
}
```

Here `projectA` uses its own Discord webhook but inherits the global
Telegram credentials. Profile credentials support `$VAR` / `${VAR}`
expansion just like global credentials. When a profile extends another,
parent credentials are merged into child (child wins on conflict).
Config validation uses merged credentials, so a `discord` step only
needs `discord_webhook` set somewhere — globally or on the profile.

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

### Webhook notifications

The `webhook` step type sends an HTTP POST to any URL with the message as the
body. Covers ntfy.sh, Pushover, Home Assistant, IFTTT, or any custom endpoint
— one step type, infinite integrations:

```json
{ "type": "webhook", "url": "https://ntfy.sh/mytopic", "text": "{Profile} build is ready", "when": "afk" }
```

The URL and optional headers live on the step itself (not in credentials), so
one config can target multiple endpoints. Custom headers can override the
default `Content-Type: text/plain` and use `$VAR` / `${VAR}` syntax for
secrets:

```json
{
  "type": "webhook",
  "url": "https://api.pushover.net/1/messages.json",
  "text": "{Profile} is ready",
  "headers": {
    "Content-Type": "application/x-www-form-urlencoded",
    "Authorization": "Bearer $PUSHOVER_TOKEN"
  },
  "when": "afk"
}
```

The `text` field supports template variables. Webhook steps run in parallel
(they don't block the audio pipeline). Requires `url` and `text` fields.

### AI voice generation

Replace robotic system TTS with high-quality AI voices. `notify voice generate`
scans the event log for frequently used `say` step messages and pre-generates
WAV files via the OpenAI TTS API. When a cached voice exists, the runner plays
it directly through the audio pipeline; otherwise it falls back to system TTS.
Everything works without an API key — AI voices are purely additive.

```json
{
  "config": {
    "openai_voice": {
      "model": "tts-1",
      "voice": "nova",
      "speed": 1.0,
      "min_uses": 3
    },
    "credentials": {
      "openai_api_key": "$OPENAI_API_KEY"
    }
  }
}
```

| Voice setting | Description |
|---------------|-------------|
| `model` | `tts-1` (fast) or `tts-1-hd` (higher quality). Default: `tts-1` |
| `voice` | `alloy`, `echo`, `fable`, `onyx`, `nova`, `shimmer`. Default: `nova` |
| `speed` | 0.25–4.0. Default: 1.0 |
| `min_uses` | Minimum event log occurrences before generating. Default: 3 |

```bash
# Generate AI voice files for frequently used say steps:
notify voice generate

# Only generate for texts used 10+ times:
notify voice generate --min-uses 10

# Play all cached voices:
notify voice play

# Play a specific cached voice:
notify voice play "Boss done"

# List cached voice files:
notify voice list

# Clear all cached voice files:
notify voice clear
```

Messages containing dynamic template variables (`{duration}`, `{time}`, `{date}`,
`{command}`, `{output}`, `{claude_*}`) cannot be pre-generated and always fall
back to system TTS. Static variables (`{profile}`, `{hostname}`) are fine.

The `notify test` dry-run shows voice source per say step:
`(ai: nova)` for cached, `(system tts)` for uncached, `(system tts, dynamic)`
for messages with runtime variables.

Cache location: `~/.config/notify/voice-cache/` (or `%APPDATA%\notify\voice-cache\`
on Windows). Files are named by SHA-256 hash of the text.

### AFK detection

Steps can be conditionally filtered with a `"when"` condition.
AFK conditions use idle time (no keyboard/mouse input); invocation
conditions distinguish `notify run` from direct calls:

| `when` value   | Step runs when... |
|----------------|-------------------|
| *(omitted)*    | Always (default, backwards compatible) |
| `"present"`    | User is **active** (idle time below threshold) |
| `"afk"`        | User is **away** (idle time at or above threshold) |
| `"run"`        | Invoked via `notify run` (command wrapper only) |
| `"direct"`     | Invoked directly or via `notify pipe` (not `notify run`) |
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

### Profile auto-selection (match rules)

When the profile argument is omitted, `notify` can auto-select the right
profile based on match rules — no extra typing needed. Add a `"match"`
object to a profile with `"dir"` and/or `"env"` conditions:

```json
{
  "profiles": {
    "work": {
      "match": { "dir": "/work/" },
      "ready": { "steps": [...] }
    },
    "personal": {
      "match": { "dir": "/hobby/", "env": "TEAM=personal" },
      "ready": { "steps": [...] }
    }
  }
}
```

- `"dir"` — substring match against the working directory (forward-slash
  normalized). `"/work/"` matches any path containing `/work/`.
- `"env"` — `KEY=VALUE` check: matches when `os.Getenv(KEY) == VALUE`.
  Empty values (`KEY=`) match when the variable is set but empty.
- All conditions are AND — both must match. For OR logic, use separate
  profiles.
- If multiple profiles match, the first alphabetically wins.
- Falls back to `"default"` when no match rule is satisfied.
- Explicit profile (`notify boss done`) always takes priority over
  auto-selection.
- `notify list` shows match rules in the output.

### Lookup logic

1. Resolve `"extends"` chains (parent actions are merged into child,
   child wins on conflict)
2. Try `profiles[profile][action]`
3. If not found, fall back to `profiles["default"][action]`
4. If neither exists, error

### Command wrapper (`notify run`)

Wrap any command to get automatic notifications on completion:

```bash
notify run -- make build              # default profile, "ready" or "error"
notify run boss -- cargo test         # boss profile
notify run -v 50 -- npm run build     # with volume override
```

`notify run` executes the command, measures its duration, then triggers
`ready` on exit code 0 or `error` on non-zero. Custom mappings in
`"exit_codes"` override this default (e.g. exit 2 → `warning`). The `--`
separator is required to distinguish notify options from the wrapped command.

Template variables available in all modes:

| Variable     | Description                          | Example                        |
|--------------|--------------------------------------|--------------------------------|
| `{profile}`  | Profile name as-is                   | `boss`                         |
| `{Profile}`  | Profile name title-cased             | `Boss`                         |
| `{time}`     | Current time (compact)               | `14:30`                        |
| `{Time}`     | Current time (spoken, for TTS)       | `2:30 PM`                      |
| `{date}`     | Current date (compact)               | `2026-02-22`                   |
| `{Date}`     | Current date (spoken, for TTS)       | `February 22, 2026`            |
| `{hostname}` | Machine hostname                     | `mypc`                         |

Additional variables available in `run` mode:

| Variable     | Description                          | Example                        |
|--------------|--------------------------------------|--------------------------------|
| `{command}`  | The wrapped command string           | `make build`                   |
| `{duration}` | Compact elapsed time                 | `2m15s`                        |
| `{Duration}` | Spoken elapsed time (for TTS)        | `2 minutes and 15 seconds`     |
| `{output}`   | Last N lines of command output       | `3 failed, 47 passed`          |

Additional variables available in `pipe` mode:

| Variable     | Description                          | Example                        |
|--------------|--------------------------------------|--------------------------------|
| `{output}`   | The matched line from stdin          | `BUILD SUCCESS`                |

Additional variables available when stdin is piped JSON (e.g. from Claude Code hooks):

| Variable           | Description                                      | Example              |
|--------------------|--------------------------------------------------|----------------------|
| `{claude_message}` | From `last_assistant_message` or `message` field  | `Build complete`     |
| `{claude_hook}`    | From `hook_event_name` field                      | `Stop`               |
| `{claude_json}`    | Full raw JSON string from stdin                   | `{"message":"..."}` |

Use `{Duration}` in `say` steps for natural speech, `{duration}` in
toast/discord/slack for compact display.

Steps can be limited to `run` mode with `"when": "run"`, or excluded
from it with `"when": "direct"`:

```json
{ "type": "say", "text": "{command} finished in {Duration}", "when": "run" },
{ "type": "say", "text": "Ready!", "when": "direct" }
```

### Output capture and pattern matching

Capture command output for use in notifications and optionally select
different actions based on output content.

**Output capture** — set `"output_lines"` in config to include the last
N lines of command output in the `{output}` template variable:

```json
{
  "config": { "output_lines": 5 },
  "profiles": {
    "default": {
      "ready": {
        "steps": [
          { "type": "discord", "text": "Done!\n{output}", "when": "afk" }
        ]
      }
    }
  }
}
```

```bash
notify run -- pytest
# Discord message: "Done!\n3 failed, 47 passed"
```

`{output}` is empty when not in `run` or `pipe` mode, or when `output_lines`
is 0 (in `run` mode). In `pipe` mode, `{output}` is always the matched line.
Output capture uses a tee — the command's stdout and stderr still print
to the terminal normally.

**Pattern matching** — use `--match` (or `-M`) to select an action based
on output content instead of exit code:

```bash
notify run --match "FAIL" error --match "passed" ready -- pytest
```

Patterns are scanned in order — first substring match wins. If no pattern
matches, the normal exit-code resolution applies (exit codes map → 0=ready,
non-zero=error). `--match` implicitly enables output capture even if
`output_lines` is 0 (but `{output}` stays empty without `output_lines`).

Action resolution order for `notify run`:
1. `--match` patterns (first substring hit wins)
2. `exit_codes` config map
3. Exit 0 → `ready`, else → `error`

### Heartbeat for long tasks

Long-running commands (30+ minute builds, deploys, test suites) give no
feedback while running — you don't know if the task hung or is still
progressing. Heartbeat fires a periodic notification so you know it's alive:

```bash
notify run --heartbeat 5m -- make build
notify run -H 2m boss -- cargo test
```

Every interval, the `"heartbeat"` action is dispatched for the resolved
profile with `{command}`, `{duration}`, and `{Duration}` set to the elapsed
time since the command started. The first tick fires after one interval (not
immediately). If the command finishes before the first tick, no heartbeat
fires.

Set a default interval in config so you don't need the flag every time:

```json
{
  "config": { "heartbeat_seconds": 300 }
}
```

The `--heartbeat` flag overrides the config value. A zero or omitted config
value means heartbeat is disabled unless the flag is passed.

Define the `"heartbeat"` action in your profile (or in `"default"`):

```json
{
  "profiles": {
    "default": {
      "heartbeat": {
        "steps": [
          { "type": "say", "text": "Still running, {Duration} elapsed", "when": "present" },
          { "type": "toast", "message": "Still running ({duration})", "when": "present" },
          { "type": "discord", "text": "Still running ({duration})", "when": "afk" }
        ]
      }
    }
  }
}
```

If the `"heartbeat"` action doesn't exist in the profile, an error is printed
to stderr but the wrapped command keeps running.

### Pipe / stream mode (`notify pipe`)

Read lines from stdin and trigger notifications when patterns match.
Useful for long-running processes you can't wrap with `notify run`:

```bash
tail -f build.log | notify pipe boss --match "SUCCESS" done --match "FAIL" error
docker compose logs -f | notify pipe ops --match "panic" error
deploy-events | notify pipe ops                    # every line triggers "ready"
```

Without `--match`, every line from stdin triggers the `"ready"` action. With
`--match`, only lines that match a pattern trigger — unmatched lines are
skipped silently. First match wins when multiple patterns could match.

The `{output}` template variable contains the matched line (the full line
from stdin that triggered the notification). Other base template variables
(`{profile}`, `{time}`, `{date}`, `{hostname}`) are available as usual.
`{command}` and `{duration}` are empty (no wrapped command).

Steps with `"when": "direct"` fire in pipe mode; steps with `"when": "run"`
do not — pipe is not a command wrapper.

For high-volume streams, use `--cooldown` (or `"cooldown": true` in config)
to prevent notification spam. Exits 0 when stdin closes (EOF).

### Stdin JSON injection (hook integration)

When stdin is piped JSON (not a terminal), `notify` auto-detects and extracts
fields as template variables. This enables seamless integration with tools like
[Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) that
pipe structured JSON to hook commands.

**How it works:** Claude Code's Stop hook pipes
`{"last_assistant_message": "...", "hook_event_name": "Stop", ...}` to stdin.
The Notification hook pipes `{"message": "...", ...}`. No flags or config changes
are needed — detection is fully automatic.

**Available variables:**

| Variable           | Source field                                 |
|--------------------|----------------------------------------------|
| `{claude_message}` | `last_assistant_message` or `message`        |
| `{claude_hook}`    | `hook_event_name`                            |
| `{claude_json}`    | Full raw JSON string                         |

**Example config** — include Claude's message in notifications:

```json
{
  "ready": {
    "steps": [
      { "type": "say", "text": "{Profile} is done. {claude_message}" },
      { "type": "discord", "text": "**{Profile}** finished at {time}\n\n{claude_message}", "when": "afk" }
    ]
  }
}
```

**Example Claude Code hook** (`.claude/settings.json`):

```json
{
  "hooks": {
    "Stop": [
      { "command": "notify done" }
    ],
    "Notification": [
      { "command": "notify attention" }
    ]
  }
}
```

When stdin is a terminal (interactive use), or when stdin is not valid JSON,
the `{claude_*}` variables expand to empty strings — existing behavior is
unchanged.

**Logging:** When `--log` is enabled, the event log summary line includes
`claude_hook=` and `claude_message=` fields so you can see which hook
triggered each notification and what message was passed.

**Dashboard:** The web dashboard's live toast popups show the hook source
(e.g. "via Stop") and the claude message text when present.

### Direct send (`notify send`)

Fire a one-off notification without defining a profile or action in config.
Takes the step type and message as positional args, pulls credentials from
the existing config:

```bash
notify send say "Build finished"                # Text-to-speech
notify send toast "Deploy complete"             # Desktop notification
notify send toast --title Deploy "All done"     # Toast with custom title
notify send telegram "Tests passed"             # Telegram message
notify send telegram_voice "Ready to review"    # Telegram voice bubble
notify send discord "Pipeline green"            # Discord message
notify send slack "Release shipped"             # Slack message
```

Supported types: `say`, `toast`, `discord`, `discord_voice`, `slack`,
`telegram`, `telegram_audio`, `telegram_voice`. Not supported: `sound`
(needs a sound name, not a message) and `webhook` (needs a URL and headers).

Template variables (`{time}`, `{date}`, `{hostname}`, etc.) are expanded
in the message text. Volume is resolved from `--volume` or the config default.

### Examples

```bash
notify ready                      # Run "ready" from the default profile
notify default ready              # Same as above (explicit default)
notify boss ready                 # Sound + speech + toast notification
notify boss done,attention        # Run "done" then "attention" from boss
notify -v 50 ready                # Run at 50% volume
notify -c myconfig.json dev done  # Use a specific config file
notify --log ready                # Log this invocation to notify.log
notify --echo ready               # Print summary: "notify: sound, say, toast"
notify --cooldown ready           # Enable cooldown for this invocation
notify send say "Build finished"  # Speak a one-off message via TTS
notify send telegram "Deploy done"  # Send directly to Telegram
notify send toast --title Build "Done"  # Toast with custom title
notify run -- make build          # Wrap a command, auto ready/error
notify run boss -- cargo test     # Wrap with a specific profile
notify run --heartbeat 5m -- make build    # Heartbeat every 5 minutes
notify run -H 2m boss -- cargo test       # Heartbeat with specific profile
notify run -M FAIL error -M passed ready -- pytest  # Match output patterns
tail -f build.log | notify pipe boss -M SUCCESS done -M FAIL error
                                  # Pipe mode: match patterns in stream
deploy-events | notify pipe ops   # Pipe: every line triggers "ready"
notify test                       # Dry-run default profile
notify test boss                  # Dry-run boss profile
notify silent 1h                  # Suppress all notifications for 1 hour
notify history                    # Show last 10 log entries
notify history 5                  # Show last 5 log entries
notify history summary            # Show action counts for last 7 days
notify history summary 30         # Show action counts for last 30 days
notify history summary all        # Show action counts for all time
notify history watch              # Live dashboard of today's activity
notify history export              # Export all log entries as JSON
notify history export 7            # Export last 7 days as JSON
notify history clean 7            # Remove entries older than 7 days
notify history clear              # Delete the log file
notify config validate            # Check config for errors
notify dashboard                  # Start web dashboard on port 8080
notify dashboard --port 9000      # Start on a different port
notify dashboard --open           # Open in a chromeless browser window
notify b ready                    # Use alias "b" for the boss profile
notify play                       # List all built-in sounds
notify play success               # Preview the success sound
notify -v 50 play blip            # Preview at 50% volume
notify voice stats                # Show all-time say text frequencies
notify voice stats 7              # Show say text frequencies for last 7 days
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

`notify history watch` shows a live dashboard that refreshes every 2 seconds.
Below the summary table it includes an hourly breakdown with one column per
profile and a `%` column showing each hour's share of the day's total — useful
for spotting your most active working hours. Press `x` or `Esc` to exit.

### Web dashboard

`notify dashboard` starts a local web UI on `http://127.0.0.1:8080` with
six tabs (linkable via URL hash, e.g. `/#watch`):

- **Watch** (default) — mirrors terminal `history watch`: summary table with
  profile/action counts, percentages, skipped, and "New" deltas since page load,
  donut charts showing notification share and time distribution per profile,
  approximate time spent per profile (gap-based estimation with 5-minute
  threshold), plus hourly breakdown with bar chart and activity timeline
  heatmap — auto-refreshes every 2 seconds. A compact
  log stats line at the bottom shows total entries, file size, and date range.
  Day navigation buttons (`<` / `>` / Today) let you browse past days; the "New"
  column only appears when viewing today
- **History** — live-updating table of notification events, fed by SSE.
  An activity chart shows stacked daily bars (green = runs, yellow = skipped)
  with hover tooltips; hidden for hour-based ranges.
  Filter dropdowns let you narrow by profile and event kind (execution,
  cooldown, silent); filters apply to both loaded entries and new SSE events.
  CSV and JSON export buttons download the filtered entries as a file
- **Config** — credential health panel showing ok/missing status per profile,
  plus read-only JSON view of your config (credentials redacted)
- **Test** — dry-run interface: pick a profile and action, see which steps
  would run without actually sending anything. The profile dropdown includes
  both config profiles and profiles extracted from the last 48h of log entries.
  Unknown profiles fall back to the `default` profile (same as the CLI).
  Template variables (`{profile}`, `{time}`, etc.) are expanded in step details
- **Voice** — say-step text frequencies from the event log, with rank, count,
  percentage, and text columns. A time-range dropdown filters by all time, 7,
  30, or 90 days
- **Silent** — view and control silent mode from the dashboard. Shows current
  status with countdown timer, quick-set buttons (15m, 30m, 1h, 2h, 4h),
  custom duration input, and disable button. A status badge appears next to
  the tab bar whenever silent mode is active

Profile names are clickable everywhere — click one to open a detail modal
showing its full step pipeline (dry-run) and credential health status.

Keyboard shortcuts: `1`–`6` switch tabs, left/right arrows navigate Watch days,
`t` jumps to today, `s` toggles screenshot mode (replaces profile names with
fake ones for privacy-safe screenshots). A dark/light theme toggle in the header
persists your preference via `localStorage`.

```bash
notify dashboard              # default port 8080
notify dashboard --port 9000  # custom port
notify dashboard --open       # launch in a chromeless browser window
```

Add `--open` to launch the dashboard in a chromeless browser window (no address
bar, no tabs) using Edge or Chrome's app mode. Falls back to the default browser
if neither is available.

The dashboard binds to `127.0.0.1` only (not exposed to the network). Config
is loaded once at startup. Press `Ctrl+C` to stop.

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
| Webhook | `net/http` (built-in) | `net/http` (built-in) | `net/http` (built-in) |

> **Note:** Development and testing has been done primarily on Windows.
> macOS and Linux support is implemented but has not been extensively
> tested yet. If you run into issues on these platforms, please
> [open an issue](https://github.com/Mavwarf/notify/issues).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and
guidelines.

## License

MIT
