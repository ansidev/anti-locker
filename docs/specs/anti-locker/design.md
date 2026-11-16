# Design: anti-locker

**Date:** 2026-08-01
**Status:** Approved; implementation integrated
**Platform:** macOS
**Language:** Go

## Intent

anti-locker is a foreground CLI that prevents idle sleep only when the current Wi-Fi SSID is in the user's configured trusted-network list. The user retains manual control over screen lock, display-off, and manual sleep.

## Goals and boundaries

- Load and validate YAML configuration, with interactive setup when the effective config file is missing.
- Detect the current Wi-Fi SSID and compare it exactly and case-sensitively with configured names.
- Run `caffeinate` only while connected to a trusted network and stop it when that condition ends.
- Shut down the child process on normal exit and SIGINT/SIGTERM.
- Keep the loop testable through narrow provider and process-manager interfaces.

Out of scope: Windows/Linux support, daemon mode, subcommands for process control, display or manual-lock inhibition, and event-driven Wi-Fi notifications.

## Architecture

`main.go` composes the CLI and runtime. `internal/config` loads, validates, and persists YAML configuration and conducts first-run setup. `internal/wifi` provides the current SSID. `internal/loop` polls the provider and applies the trusted-network state machine through `internal/keepawake.Manager`. `internal/keepawake` verifies and manages the `caffeinate` child process.

Dependency direction:

```text
main.go
  └── internal/loop
      ├── internal/config
      ├── internal/wifi
      └── internal/keepawake
```

The loop depends on the `wifi.Provider` and `keepawake.Manager` interfaces, so unit tests use in-memory fakes. Production wiring uses `wifi.ExecProvider` and `keepawake.ExecManager`.

## Configuration and CLI

The CLI is named `anti-locker`, uses `urfave/cli/v3`, and offers `--config`, `--help`, and `--version`. Its default config path is `~/.config/anti-locker.yaml`; a leading `~/` in an override is expanded before file access.

The YAML config has `interval` (default 3600 seconds), required `networks` (an explicit empty list is allowed for an existing config), and `caffeinate_arg` (default `-i`). Loading validates positive interval, non-empty network entries, and non-empty caffeinate argument; it trims SSIDs and de-duplicates them with a warning. First-run setup requests the interval and at least one network, then writes the config with restrictive file permissions.

## Wi-Fi lookup

`wifi.Provider` exposes `CurrentSSID() (string, error)`. The production provider currently uses `github.com/jaisonerick/macwifi` to scan for the current network. If the helper launch fails, it clears the helper extraction cache and retries once. If that still fails, it falls back to `ipconfig getsummary` using the detected Wi-Fi interface (or `en0` if interface discovery is unavailable). Lookup failures are returned to the loop as transient errors. This is the implemented recovery path; it supersedes the original spec's simpler `ipconfig`-then-`system_profiler` approach.

## Keep-awake process lifecycle

`keepawake.Manager` exposes `Start(network, caffeinateArg)`, `Stop()`, and `IsRunning()`. `ExecManager` starts `caffeinate` with the configured argument, tracks and reaps the child process, and is safe to stop repeatedly. `Stop()` sends SIGTERM, waits briefly, then sends SIGKILL if the process has not exited. `main.go` also defers cleanup so the child cannot outlive anti-locker.

## Runtime state machine

The loop performs one SSID check immediately, then checks on each configured ticker interval:

- Trusted SSID and no child running: start `caffeinate` with the configured argument and log the transition.
- Untrusted or empty SSID and child running: stop `caffeinate` and log the transition.
- No required state change: leave the child state alone.
- SSID lookup or start failure: log a warning and retry on a later tick.
- Context cancellation: stop `caffeinate` and return.

`main.go` wires SIGINT and SIGTERM to context cancellation. Fatal config errors and missing `caffeinate` are reported before the loop starts.

## Validation and integration seams

Unit tests use temporary config files, scripted SSID providers, and fake process managers. Wi-Fi recovery helpers are independently testable through injected scan, heal, and fallback functions. An opt-in integration test exercises real SSID detection and is skipped in short mode. Repository quality commands are `go build ./...`, `go test ./...`, `go test -short ./...`, `go vet ./...`, `gofmt -s -l .`, and a Darwin build.

## Implementation alignment

The approved 2026-08 design specified `ipconfig` with `system_profiler` fallback and a one-argument keep-awake start operation. The current implementation uses the `macwifi` recovery path with `ipconfig` fallback and allows `caffeinate_arg` configuration. These implementation details are recorded here while preserving the original product boundaries.

One observable behavior still differs from the approved design: `internal/loop` logs the connected SSID on every unmatched check, while the approved design calls for no-op iterations to remain silent. Task 7 records this unresolved alignment item; this documentation migration does not change runtime behavior.
