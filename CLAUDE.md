# CLAUDE.md

## Build & test

```bash
go build -o output/notify.exe ./cmd/notify   # build binary
go vet ./...                                  # lint
go test ./...                                 # run all tests
```

## Documentation rules

When adding or changing user-facing features, update all of these together:
- `cmd/notify/main.go` `printUsage()` — help output
- `README.md` — Options table, config format example, and prose description
- `HISTORY.md` — features list at top + detailed `###` entry under today's date

## Adding a new CLI flag (opt-in bool)

Follow the established pattern:
1. Add field to `Options` struct in `internal/config/config.go`
2. In `cmd/notify/main.go`: add flag var, parse in flag switch, add `shouldX()` helper
3. Wire into both `runAction` and `runWrapped` (they must stay in sync)
4. Add to `printUsage()` Options section

## Adding a new step type

1. Create `internal/<type>/<type>.go` with the implementation
2. Add to `validStepTypes` in `internal/config/config.go`
3. Add required-field validation in `Validate()`
4. Add dispatch case in `internal/runner/runner.go`
5. Add step detail case in `internal/eventlog/eventlog.go`
6. Update `Step` struct comment listing all types

## Conventions

- Commit messages: imperative mood, short first line, body for "why"
- HISTORY.md features list: one-liner per feature with `*(Mon DD)*` date
- Config validation catches errors at load time, not at execution
- Remote steps (discord, slack, telegram, webhook) run in parallel; audio steps (sound, say) run sequentially
