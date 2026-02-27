# TODO

## Medium Impact

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
