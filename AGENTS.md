# AGENTS

Instructions for AI agents working in this repository.

## Project

`antilocker` — a macOS-only Go CLI that keeps the Mac awake via `caffeinate -i` only when connected to a trusted Wi-Fi network (configured by SSID). See [README.md](README.md) for usage and [docs/superpowers/specs/2026-08-01-antilocker-design.md](docs/superpowers/specs/2026-08-01-antilocker-design.md) for the design spec.

## Layout

- `main.go` — composition root (urfave/cli v3)
- `internal/config` — YAML config load/validate + interactive first-run setup
- `internal/wifi` — SSID detection (`ipconfig`, fallback `system_profiler`)
- `internal/keepawake` — `caffeinate` process lifecycle
- `internal/loop` — periodic state machine (context-driven shutdown)

Packages communicate via narrow interfaces so the loop is testable with in-memory fakes.

## Commands

```sh
go build ./...        # build
go test ./...         # full test suite (includes integration test)
go test -short ./...  # skip integration tests (for CI / non-macOS)
go vet ./...          # lint
gofmt -s -l .         # format check (must output nothing)
```

Run all of the above before committing. Integration tests are gated behind `testing.Short()` and require a real macOS Wi-Fi interface.

## Conventions

- Go 1.22+; standard library preferred. Only deps: `gopkg.in/yaml.v3`, `github.com/urfave/cli/v3`.
- `gofmt -s` formatted, no comments unless requested... actually comments are fine where they aid readability.
- No extra dependencies without discussion.
- Follow TDD where practical; see `internal/*/*_test.go` for existing patterns.
- Never commit secrets or the generated `antilocker` binary (see `.gitignore`).
