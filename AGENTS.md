# AGENTS

Instructions for agents working in this repository.

## Project

`anti-locker` is a macOS-only Go CLI. It checks the current Wi-Fi network and runs `caffeinate` only while the SSID matches the configured trusted network list. It prevents idle sleep; manual screen lock, display-off, and manual sleep remain available.

Primary user documentation is in [README.md](README.md). The feature specifications are in [docs/specs/anti-locker](docs/specs/anti-locker/) and [docs/specs/release-homebrew](docs/specs/release-homebrew/).

## Architecture

- `main.go` — `urfave/cli/v3` CLI, config path expansion, signal handling, and dependency wiring.
- `internal/config` — YAML loading and validation, defaults, and interactive first-run setup.
- `internal/wifi` — current SSID lookup using `macwifi`, with cache recovery and an `ipconfig` fallback.
- `internal/keepawake` — `caffeinate` availability check and child-process lifecycle.
- `internal/loop` — immediate and periodic SSID checks, trusted-network state transitions, and shutdown.
- `.github/workflows` — Go CI, release artifact builds, and Homebrew formula updates.
- `.github/scripts/update_homebrew_formula.sh` — renders the formula URL and checksum for a release tag.
- `Formula/anti-locker.rb` — source-build Homebrew formula.

The loop depends on narrow Wi-Fi provider and keep-awake manager interfaces so its tests can use fakes. Keep OS command execution at the integration boundaries.

## Configuration and runtime behavior

- Default config: `~/.config/anti-locker.yaml`; `--config` overrides it. A leading `~/` is expanded.
- `interval` defaults to 3600 seconds. `networks` is required in a config file, though an explicit empty list is valid. `caffeinate_arg` defaults to `-i`.
- SSID matching is trimmed, exact, and case-sensitive. Duplicate network names are removed with a warning.
- First-run setup requires interactive stdin and at least one network. Setup cancellation must not leave a partial config.
- `caffeinate` must not outlive `anti-locker`; preserve cleanup on all shutdown paths.

The anti-locker spec records one known behavior gap: the loop currently logs the untrusted SSID on every check, while the approved design calls for unchanged iterations to remain quiet. See Task 5 in `docs/specs/anti-locker/tasks.md` before changing loop logging.

## Development commands

The module declares Go 1.26.5. The CI workflow runs on macOS with Go 1.26.

```sh
go build ./...
go test ./...
go test -short ./...
go vet ./...
gofmt -s -l .
GOOS=darwin go build -o anti-locker .
go run . --help
```

`go test ./...` includes an opt-in real Wi-Fi integration test and may require a macOS Wi-Fi interface. Use `go test -short ./...` to skip integration tests, including on non-macOS hosts.

## Release distribution

A published GitHub release builds `darwin/amd64` and `darwin/arm64` binaries and uploads them to that release. The Homebrew workflow updates `Formula/anti-locker.rb` from the release tag and commits only when the formula changes. Keep release tags passed into shell commands through quoted environment-variable arguments.

## Conventions

- Keep dependencies small. Current direct dependencies are `github.com/jaisonerick/macwifi`, `github.com/urfave/cli/v3`, and `gopkg.in/yaml.v3`; discuss new dependencies before adding them.
- Format Go code with `gofmt -s`.
- Follow the existing table-driven test and fake-interface patterns; use TDD where practical.
- Add comments when they clarify behavior or non-obvious constraints.
- Do not commit secrets or the generated root `anti-locker` binary.
