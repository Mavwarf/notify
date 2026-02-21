# TODO

### TTS Audio Export / Discord Voice Messages

Generate TTS as an audio file instead of (or in addition to) playing it
through speakers. All platform TTS engines support file output:

- Windows: `SpeechSynthesizer.SetOutputToWaveFile()`
- macOS: `say -o output.aiff` (then `afconvert` to WAV)
- Linux: `espeak-ng --stdout > output.wav`

**Plan:** Add `speech.SayToFile(text, volume, path)` on each platform,
then `discord.SendVoice(webhookURL, wavPath, text)` using multipart POST.
New `discord_voice` step type generates TTS to temp file, uploads to
Discord, and cleans up.

### Telegram Voice Messages

Send TTS audio to Telegram chats. Telegram's `sendVoice` requires
OGG/OPUS format for the voice bubble UX; `sendAudio` accepts WAV but
displays as an audio player instead.

Two approaches:

- **`telegram_audio`**: Use `sendAudio` with WAV — no conversion needed,
  plays inline but not as a voice bubble.
- **`telegram_voice`**: Convert WAV → OGG/OPUS via `ffmpeg`, then use
  `sendVoice` for native voice bubble. Requires `ffmpeg` as external
  dependency.

Start with `telegram_audio` (no deps), add `telegram_voice` later as
opt-in when `ffmpeg` is available.

### More Remote Notification Actions

Additional step types beyond `discord` and `telegram`:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `slack`    | POST to Slack webhook                | All (net/http) |
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli |

Would extend `"config"` → `"credentials"` with additional webhook URLs
and API tokens. Same pattern as the existing discord/telegram integration.
