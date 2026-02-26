# TODO

### MQTT Publish

Publish a message to an MQTT broker topic. Useful for home automation —
e.g. flash a desk light when a build finishes, or trigger any
Home Assistant automation via MQTT. Config would need broker URL, topic,
and optional auth in credentials. Payload could use template variables
like other steps.

### More Remote Notification Actions (low priority)

Additional step types beyond `discord`, `slack`, and `telegram`:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli + Java |

Would extend `"config"` → `"credentials"` with additional webhook URLs
and API tokens. Same pattern as the existing discord/slack/telegram integration.

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

### Modern Toast Notifications (Windows)

Upgrade from `NotifyIcon.BalloonTip` to the Windows 10+ `ToastNotificationManager`
XML API. This enables rich text formatting via `hint-style` on `<text>` elements
inside `<group><subgroup>` blocks — including bold (`base`), subtitle, title,
header sizes, and subtle (60% opacity) variants. Could also support hero images,
attribution text, action buttons, and progress bars.

Would require rewriting `internal/toast/toast_windows.go` to build XML templates
and use `Windows.UI.Notifications.ToastNotificationManager` via PowerShell.
Linux and macOS toast implementations would remain unchanged.

### `notify watch` — File Watching (stretch)

PID watching is done (`notify watch --pid <PID>`). File watching
(`notify watch --file build.log`) remains a possibility — trigger when
a file is modified. Would need `fsnotify` or polling with `os.Stat`.

### Plugin System

External scripts as step types. A `"plugin"` step would run a user
script with template variables as environment variables, enabling
integration with any tool without adding it to the binary.

### Chained Actions (`on_success` / `on_failure`)

Actions that trigger other actions based on step outcomes. E.g.
`"on_failure": "escalate"` would run the `escalate` action if any
step in the current action fails. Enables retry and escalation
patterns without shell scripting.

### AI Voice Generation (`notify voice`)

Replace robotic system TTS (Windows SAPI, macOS `say`) with pre-generated
AI voice files. Most `say` step messages are static or near-static — generate
them once via an API and cache as WAV files for instant high-quality playback.

**Provider options:**

| Provider | WAV support | Cost | Free tier | Quality |
|----------|-------------|------|-----------|---------|
| OpenAI TTS | Native (`response_format: "wav"`) | $15–30/1M chars | $5 signup credit | Very good |
| ElevenLabs | MP3/PCM (needs conversion) | ~$180/1M chars | 10k chars/month | Best-in-class |

OpenAI TTS is the pragmatic default: native WAV output (no ffmpeg needed),
10–100x cheaper, simple REST API. ElevenLabs has the best quality but costs
significantly more. Could support both behind a provider abstraction.

**Subcommands:**

- `notify voice generate` — scan all `say` steps across all profiles,
  expand static template variables, call the TTS API for any text that
  doesn't already have a cached WAV, save to voice cache directory.
  Shows a summary of generated / skipped / failed lines.
- `notify voice list` — show all cached voice files with the text they
  contain, file size, and generation date. Highlights lines that are
  missing from the cache (would fall back to system TTS).
- `notify voice clear` — delete all cached files (or for a specific
  profile).
- ~~`notify voice stats`~~ — done (shows say-step text frequencies from
  event log; dashboard Voice tab also available).

**Playback integration:**

When a `say` step runs, check the voice cache for a pre-generated WAV
matching the expanded text. If found, play the WAV file directly (same
audio pipeline as `sound` steps). If not found, fall back to the existing
system TTS engine. This means AI voices are purely additive — everything
works without an API key, just with lower quality.

**Cache design:**

- Location: notify data directory (`voices/` subfolder alongside
  `notify.log`, `cooldown.json`) or configurable via
  `"voice_cache": "/path"` in config.
- File naming: hash of the expanded text (e.g. `sha256[:16].wav`) so
  the same text always maps to the same file regardless of which
  profile/action uses it. A `voices.json` index maps hash → original
  text, voice name, provider, and generation timestamp.
- Template variables like `{profile}`, `{duration}`, `{time}` produce
  different text at runtime — these lines can't be pre-generated and
  always fall back to system TTS. `notify voice generate` should warn
  about these.

**Config additions:**

```json
"config": {
  "voice": {
    "provider": "openai",
    "model": "tts-1",
    "voice": "nova",
    "speed": 1.0
  },
  "credentials": {
    "openai_api_key": "$OPENAI_API_KEY"
  }
}
```

**`notify test` integration:**

The dry-run output should show per-`say` step whether a cached AI voice
file exists (e.g. `[say] "Build complete" (ai: nova)` vs
`[say] "Finished in {duration}" (system tts, dynamic)`).

**Implementation notes:**

- New package `internal/voice/` for provider abstraction, cache
  management, and WAV generation.
- OpenAI TTS: POST to `https://api.openai.com/v1/audio/speech` with
  `model`, `input`, `voice`, `response_format: "wav"`. Response body
  is the raw WAV file. Simple `net/http` — no SDK needed.
- ElevenLabs: POST to `https://api.elevenlabs.io/v1/text-to-speech/{voice_id}`
  with `output_format: "pcm_44100"`. Would need WAV header wrapping.
- Reuse `internal/httputil` for HTTP client with timeout and retry.
- Cache lookup happens in `internal/speech/` before falling back to
  the platform TTS engine.

