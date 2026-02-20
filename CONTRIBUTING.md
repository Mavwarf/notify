# Contributing to notify

Thanks for your interest in contributing!

## Getting started

1. Fork the repo and clone your fork
2. Install [Go 1.24+](https://go.dev/dl/)
3. Run the tests:
   ```bash
   go test ./...
   ```
4. Build:
   ```bash
   go build -o notify ./cmd/notify
   ```

## Making changes

1. Create a branch from `main`
2. Make your changes
3. Run `go vet ./...` and `go test ./...`
4. Commit with a clear message — short summary line, optional body explaining the "why"
5. Open a pull request against `main`

## Pull request guidelines

- Keep PRs focused — one feature or fix per PR
- Add tests for new functionality when possible
- Update `README.md` if your change affects user-facing behavior
- CI must pass (tests + cross-platform build check)

## Project structure

```
cmd/notify/          CLI entry point
internal/
  audio/             Sound synthesis and playback
  config/            Config loading and resolution
  discord/           Discord webhook integration
  eventlog/          Invocation logging
  idle/              Platform-specific AFK detection
  runner/            Step execution engine
  shell/             Shell escaping utilities
  speech/            Text-to-speech (per-platform)
  tmpl/              Template variable expansion
  toast/             Desktop notifications (per-platform)
```

## Platform-specific code

Platform code uses Go filename conventions (`_windows.go`, `_darwin.go`, `_linux.go`). If your change touches platform-specific behavior, note which platforms you tested.

## Reporting bugs

Use the [bug report template](https://github.com/Mavwarf/notify/issues/new?template=bug_report.md) on GitHub Issues.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
