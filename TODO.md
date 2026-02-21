# TODO

### TTS Audio Export / Discord Voice Messages

Generate TTS as an audio file instead of (or in addition to) playing it
through speakers. All platform TTS engines support file output:

- Windows: `SpeechSynthesizer.SetOutputToWaveFile()`
- macOS: `say -o output.aiff`
- Linux: `espeak-ng --stdout > output.wav`

This enables uploading voice messages to Discord (multipart POST via
webhook API), attaching audio to Slack/Telegram/email, or piping to
any external tool.

Two possible approaches:

- **Built-in**: A `discord_voice` step type that generates TTS and
  uploads the WAV to Discord in one go.
- **Generic**: A `"say"` option like `"output": "file"` that writes
  a WAV, then let other steps or external tools consume it.

Generic approach is more flexible but more complex. Could start with
the built-in approach and generalize later.

### More Remote Notification Actions

Additional step types beyond the existing `discord` webhook support:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `slack`    | POST to Slack webhook                | All (net/http) |
| `telegram` | POST to Telegram Bot API             | All (net/http) |
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli |

Would extend `"config"` → `"credentials"` with additional webhook URLs
and API tokens. Same pattern as the existing discord integration.
