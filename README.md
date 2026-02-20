# notify

**Never miss a finished build again.** Whether you're at your desk or grabbing
coffee — `notify` knows and reaches you the right way: a chime when you're
present, a Discord ping when you're not.

A single binary, zero-dependency notification engine for the command line.
Chain sounds, speech, toast popups, and Discord messages into pipelines —
all configured in one JSON file.

## What is this for?

Long-running terminal commands finish silently. `notify` gives you instant
feedback:

```bash
make build && notify ready || notify error
```

Or notify yourself when a deployment completes:

```bash
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
  combines sound, speech, toast, and Discord steps.
- **Built-in sounds** — 7 generated tones (success, error, warning, etc.)
  created programmatically as sine-wave patterns.
- **Text-to-speech** — uses OS-native TTS engines
  (Windows SAPI, macOS `say`, Linux `espeak`).
- **Toast notifications** — native desktop notifications on all platforms
  (Windows Toast API, macOS `osascript`, Linux `notify-send`).
- **Discord webhooks** — post messages to a Discord channel via webhook,
  no external dependencies (just `net/http`).
- **AFK detection** — conditionally run steps based on whether the user is
  at their desk or away. Play a sound when present, send a Discord message
  when AFK.
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
    config.go            Config loading and profile/action resolution
  discord/
    discord.go           Discord webhook integration (POST to channel)
  idle/
    idle_windows.go      User idle time via GetLastInputInfo (Win32)
    idle_darwin.go       User idle time via ioreg HIDIdleTime
    idle_linux.go        User idle time via xprintidle
  runner/
    runner.go            Step executor (dispatches to audio/speech/toast/discord)
  eventlog/
    eventlog.go          Append-only invocation log (~/.notify.log)
  tmpl/
    tmpl.go              Template variable expansion ({profile}, {Profile})
  shell/
    escape_windows.go    PowerShell string escaping
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
notify list                            # List all profiles and actions
notify version                         # Show version and build date
notify help                            # Show help
```

### Options

| Flag               | Description                              |
|--------------------|------------------------------------------|
| `--volume`, `-v`   | Override volume, 0-100 (default: config or 100) |
| `--config`, `-c`   | Path to notify-config.json               |

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
    "credentials": {
      "discord_webhook": "https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN"
    }
  },
  "profiles": {
    "default": {
      "ready": {
        "steps": [
          { "type": "sound", "sound": "success" },
          { "type": "say", "text": "Ready!", "when": "present" },
          { "type": "toast", "message": "Ready!", "when": "afk" },
          { "type": "discord", "text": "Ready!", "when": "afk" }
        ]
      }
    },
    "boss": {
      "ready": {
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
  `toast` (desktop notification), `discord` (post to Discord channel via webhook).
- **Volume priority:** per-step `volume` > CLI `--volume` > config
  `"default_volume"` > 100.
- Toast `title` defaults to the profile name if omitted.
- **Template variables:** use `{profile}` in `say` text, `toast` title/message,
  or `discord` text to inject the runtime profile name, or `{Profile}` for
  title case (e.g. `boss` → `Boss`). This is especially useful with the
  default fallback — a single action definition can produce different messages
  depending on which profile name was passed on the CLI.
- `sound` and `say` steps run sequentially (shared audio pipeline).
  All other steps (`toast`, `discord`, etc.) fire in parallel immediately.

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

Remote notification steps (like `discord`) need credentials stored in the
`"credentials"` object inside `"config"`:

```json
{
  "config": {
    "credentials": {
      "discord_webhook": "https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN"
    }
  },
  "profiles": { ... }
}
```

To get a Discord webhook URL: Server Settings → Integrations → Webhooks →
New Webhook → Copy Webhook URL.

### Discord notifications

The `discord` step type posts a message to a Discord channel via webhook.
Especially useful with `"when": "afk"` to reach you when you're away:

```json
{ "type": "discord", "text": "{Profile} build is ready", "when": "afk" }
```

The `text` field supports template variables (`{profile}`, `{Profile}`).
Discord steps run in parallel (they don't block the audio pipeline).

### AFK detection

`notify` checks how long the user has been idle (no keyboard/mouse input)
and uses that to filter steps with a `"when"` condition:

| `when` value | Step runs when... |
|--------------|-------------------|
| *(omitted)*  | Always (default, backwards compatible) |
| `"present"`  | User is **active** (idle time below threshold) |
| `"afk"`      | User is **away** (idle time at or above threshold) |

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

### Lookup logic

1. Try `profiles[profile][action]`
2. If not found, fall back to `profiles["default"][action]`
3. If neither exists, error

### Examples

```bash
notify ready                      # Run "ready" from the default profile
notify default ready              # Same as above (explicit default)
notify boss ready                 # Sound + speech + toast notification
notify -v 50 ready                # Run at 50% volume
notify -c myconfig.json dev done  # Use a specific config file
```

### Event log

Every `notify` invocation is logged to `~/.notify.log` for history and
debugging. Only steps that actually ran are logged (steps filtered out by
AFK detection are omitted). A blank line separates each invocation:

```
2026-02-20T14:30:05+01:00  profile=boss  action=ready  steps=sound,say,toast  afk=false
2026-02-20T14:30:05+01:00    step[1] sound  sound=notification
2026-02-20T14:30:05+01:00    step[2] say  text="Boss is ready"
2026-02-20T14:30:05+01:00    step[3] toast  title="Boss" message="Ready to go"

2026-02-20T14:35:12+01:00  profile=default  action=ready  steps=sound,toast  afk=true
2026-02-20T14:35:12+01:00    step[1] sound  sound=success
2026-02-20T14:35:12+01:00    step[2] toast  title="AFK" message="Ready!"
```

Template variables (`{profile}`, `{Profile}`) are expanded in the log so you
see the actual text that was spoken or displayed. Logging is best-effort —
errors are printed to stderr but never fail the command.

## Building

### Prerequisites

- [Go](https://go.dev/dl/) 1.24 or later
- [CMake](https://cmake.org/download/) 3.16 or later (optional — you can also
  use `go build` directly)

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions and
guidelines.

## License

MIT
