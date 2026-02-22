# TODO

### Telegram Voice Bubble (`telegram_voice`)

Convert WAV → OGG/OPUS via `ffmpeg`, then use Telegram's `sendVoice`
for native voice bubble UX. Requires `ffmpeg` as external dependency.
`telegram_audio` (WAV via `sendAudio`, inline audio player) is already
implemented.

### More Remote Notification Actions

Additional step types beyond `discord` and `telegram`:

| Type       | Description                          | Platform notes |
|------------|--------------------------------------|----------------|
| `slack`    | POST to Slack webhook                | All (net/http) |
| `email`    | Send email via SMTP                  | All (net/smtp) |
| `signal`   | Send via signal-cli                  | Needs signal-cli |

Would extend `"config"` → `"credentials"` with additional webhook URLs
and API tokens. Same pattern as the existing discord/telegram integration.
