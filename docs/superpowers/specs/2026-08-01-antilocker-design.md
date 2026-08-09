# Anti Locker — Design Specification

**Date:** 2026-08-01
**Status:** Approved
**Author:** Brainstorming session with user

## 1. Overview

**Anti Locker** is a macOS-only Go CLI that keeps the user's MacBook awake by preventing idle sleep **conditionally**, based on the currently connected Wi-Fi network. When the Mac is connected to a Wi-Fi network listed in the user's config, the app runs `caffeinate -i` to prevent idle sleep. When not, it does nothing. Manual screen lock and display-off remain fully under user control at all times.

- Binary name: `antilocker`
- Target platform: macOS only
- Language: Go (minimum toolchain: Go 1.22)
- Dependencies: `gopkg.in/yaml.v3` (YAML config), `urfave/cli` **v3** (CLI framework)

## 2. User Workflow

1. Install `antilocker` binary.
2. Run `antilocker` for the first time → first-run interactive setup prompts for interval and networks → saves config → starts running.
3. Subsequent runs: `antilocker` loads config and starts monitoring immediately.
4. App runs in the foreground; press `Ctrl-C` to stop. On exit, `caffeinate` is terminated cleanly.

## 3. Behavior

### 3.1 Core loop (running mode)

- Checks current Wi-Fi SSID every `interval` seconds (from config).
- If SSID matches any entry in `networks` → start persistent `caffeinate -i` (prevent idle sleep only).
- If SSID does not match or Wi-Fi is off → stop `caffeinate` if it was running.
- Runs one check immediately at startup (before first tick).
- State transitions are logged with timestamps; no-op iterations are silent.

### 3.2 Manual lock/display-off

By design, `caffeinate -i` only blocks *idle system sleep*. The user can still:

- Lock the screen manually (`Ctrl+Cmd+Q`)
- Turn off the display manually
- Put the Mac to sleep manually

### 3.3 First-run setup (config file missing)

**Path rule:** Setup writes to the **effective** config path (default `~/.config/antilocker.yaml` or the `--config` override). The printed `<path>` below is always this effective path.

1. Print: `No config found at <path> — let's set one up.`
2. Prompt: `Check interval in seconds [3600]:` — empty input defaults to `3600`. Non-numeric or `<= 0` values cause re-prompt.
3. Prompt: `Add a network name (leave empty to finish):` — repeats until empty input. At least one network is required; loops until one is provided.
4. Create parent directories if needed, write the YAML config, print the saved path.
5. Continue directly into the normal run loop with the new config; no restart is required. If the process receives SIGTERM during setup, it cancels identically to Ctrl-C (SIGINT).

If stdin is not a TTY (e.g. stdin is piped or closed), first-run setup cannot prompt. The app prints a clear message asking the user to run `antilocker` interactively and exits with code `1`.

### 3.4 Ctrl-C / SIGTERM during first-run setup

- Prints `Setup cancelled.`
- Does **not** write a partial config file.
- Exits with code `1`.

## 4. Configuration

### 4.1 File location

- Default: `~/.config/antilocker.yaml`
- Override: `antilocker --config /path/to/config.yaml`
- `~` is expanded at startup.

### 4.2 Format

```yaml
interval: 3600            # optional, seconds; defaults to 3600
networks:                 # required key (may be an empty list)
  - "Home Network 1"
  - "Home Network 2"
```

**First-run write guarantee:** the interactive setup always writes `interval` and `networks` explicitly (never relies on the YAML default for `interval`).

### 4.3 Config loading rules

| Condition | Behavior |
|---|---|
| File missing | Trigger first-run setup (section 3.3) |
| Malformed YAML (syntax **or** wrong types — e.g. `interval: high`, `networks: "foo"`) | Print parse/type error, exit code `1` |
| `networks: null` / `networks: ~` | Validation error ("networks key is required"), exit code `1` |
| `interval` present but `<= 0` | Validation error, exit code `1` (non-numeric strings are a YAML type error) |
| `networks` key missing entirely **or** explicitly `null`/`~` | Validation error, exit code `1` |
| Any `networks` entry empty after trimming | Validation error, exit code `1` |
| Config file exists but unreadable (permissions / is-a-directory) | Read error message, exit code `1` |
| Duplicate SSIDs | De-duplicate, print startup warning |
| `networks: []` (explicitly empty) | Valid; app runs, matches nothing |
| SSID whitespace | Trimmed at load time |

### 4.4 Network matching

- Match is **exact** and **case-sensitive** against the trimmed current SSID.

## 5. Architecture

Single Go module, flat layout:

```
.
├── main.go                      # urfave/cli app setup, flags, version
├── internal/
│   ├── config/
│   │   ├── config.go            # Config struct, Load(), validation, Contains()
│   │   ├── config_test.go
│   │   ├── firstrun.go          # Interactive first-run setup prompts
│   │   └── firstrun_test.go
│   ├── wifi/
│   │   ├── wifi.go              # CurrentSSID(), ipconfig/system_profiler parsers
│   │   └── wifi_test.go
│   ├── keepawake/
│   │   ├── keepawake.go         # caffeinate process lifecycle (Start/Stop/IsRunning)
│   │   └── keepawake_test.go
│   └── loop/
│       ├── loop.go              # Periodic state machine, signal handling
│       └── loop_test.go
```

### 5.1 Dependency direction

```
main.go
  └──> internal/loop
        ├──> internal/config
        ├──> internal/wifi
        └──> internal/keepawake
```

Nothing depends back on the loop. Packages communicate via narrow Go interfaces so `loop` is fully testable with in-memory fakes (no real `exec` in unit tests).

## 6. Components

### 6.1 CLI (`main.go`) — `urfave/cli`

- Single command, no subcommands.
- `--config <path>` — override default config path.
- `--version` — built-in version output.
- Help text shows defaults and an example config snippet.

### 6.2 Config (`internal/config`) — `gopkg.in/yaml.v3`

```go
type Config struct {
    Interval int      `yaml:"interval"`
    Networks []string `yaml:"networks"`
}
```

- `Load(path string) (*Config, error)` — read file, `yaml.Unmarshal`, validate, return. Returns `os.ErrNotExist` when the file is missing so the caller can trigger first-run setup. (`~` is expanded by the caller before `Load` is invoked.)
- `Contains(ssid string) bool` — trimmed, exact, case-sensitive match.
- Validation: `networks` key must exist; `interval > 0` when present; duplicate SSIDs de-duplicated with warning.

#### First-run setup (`firstrun.go`)

- `Run(path string) (*Config, error)` — interactive prompts reading from stdin line-by-line.
- Persists validated YAML to `path` (creating parent dirs as needed) and returns the newly created config.

### 6.3 Wi-Fi (`internal/wifi`)

Package-level discovery functions plus the injectable seam used by the loop:

```go
// Provider is the seam injected into internal/loop for testing.
type Provider interface {
    CurrentSSID() (string, error)
}

// ExecProvider is the production implementation; it shells out to macOS.
// lookPath is injectable so unit tests never touch real exec.
// Production wiring passes exec.LookPath.
type ExecProvider struct{ lookPath func(string) (string, error) }
func NewExecProvider(lookPath func(string) (string, error)) *ExecProvider
func (p *ExecProvider) CurrentSSID() (string, error)
```

Exec seam: `ExecProvider` takes an injectable `lookPath func(string) (string, error)` so unit tests never touch real `exec` either. The production wiring uses `exec.LookPath`.

`ExecProvider.CurrentSSID()` behavior:

1. `ipconfig getsummary en0` — parse the `  SSID : <value>` line.
2. Fallback: `system_profiler SPAirPortDataType` — in the `Current Network Information:` block, parse the first network name line.
3. Empty SSID (Wi-Fi off / not connected) returns `""` with `nil` error.

Note: if the `ipconfig` or `system_profiler` binaries themselves are missing (`exec.ErrNotFound`), `CurrentSSID` returns the lookup error. The loop logs it as a transient warning and retries next tick — only `caffeinate` (checked by `VerifyCaffeinate` at startup) is treated as fatal.

Parsing functions are pure and unit-tested against realistic fixture outputs.

### 6.4 Keep-awake (`internal/keepawake`)

```go
// Manager is the seam injected into internal/loop for testing.
type Manager interface {
    Start(network string) error   // no-op if already running
    Stop()                        // no-op if already stopped
    IsRunning() bool
}

// ExecManager is the production implementation backed by os/exec.
// VerifyCaffeinate is called once from main() before loop.Run —
// NOT from inside loop.Run — so loop tests never touch real exec.
func VerifyCaffeinate() error
func NewExecManager() *ExecManager
```

`ExecManager` behavior:

- Wraps `os/exec`. Spawns persistent `caffeinate -i` as a child process.
- `Start` — no-op if already running.
- `Stop()` — SIGTERM → short grace period → SIGKILL; safe to call multiple times.
- `Stop()` is registered via `defer` in the caller and invoked on SIGINT/SIGTERM, so `caffeinate` never outlives `antilocker`.
- `caffeinate` output is discarded.

### 6.5 Loop (`internal/loop`)

```go
// Run executes the periodic check loop until ctx is cancelled.
// It has no return value: context cancellation / SIGINT/SIGTERM is normal
// shutdown (exit 0 handled by caller), and all recoverable runtime issues
// are logged as warnings. Fatal configuration problems are caught before
// Run is invoked.
func Run(ctx context.Context, cfg *config.Config, w wifi.Provider, k keepawake.Manager)

1. Log startup summary: config path, networks, interval.
2. Run one check immediately; set initial caffeinate state.
3. On every `time.Ticker` tick: `w.CurrentSSID()` → `cfg.Contains(ssid)`:
   - match && !running → `k.Start(ssid)` → log
   - !match && running → `k.Stop()` → log
   - otherwise → silent no-op
4. On `ctx.Done()` (SIGINT/SIGTERM are wired to `context.WithCancel` by the caller): `k.Stop()`, return. Caller exits `0`.
5. Transient errors (SSID lookup failure, caffeinate start/stop failure) are logged as timestamped warnings; the loop continues and retries next tick.

Note: `VerifyCaffeinate()` is called from `main.go` between `config.Load` and `loop.Run` — never inside `Run` — preserving the no-real-`exec` seam for loop tests.

## 7. Data Flow

```
main.go (urfave/cli)
      │
      ▼
internal/config.Load(path)
      │
      ├── file missing ─► first-run setup (prompts) ──► save YAML ──► *Config
      │                                                          │
      ▼                                                          │
*Config ◄─────────────────────────────────────────────────────────┘
      │
      ▼
internal/keepawake.VerifyCaffeinate()      (fatal if missing)
      │
      ▼
internal/loop.Run(ctx, cfg, deps)
      │
      ├── log startup summary (path, networks, interval)
      │
      ├── immediate check
      │       │
      │       ▼
      │   wifi.CurrentSSID()
      │       ├─► ipconfig getsummary en0          (primary)
      │       └─► system_profiler SPAirPortDataType  (fallback)
      │       │
      │       ▼
      │   cfg.Contains(ssid)?
      │       ├─► yes & !running ──► keepawake.Start(ssid) ──► log
      │       ├─► no  & running  ──► keepawake.Stop()      ──► log
      │       └─► otherwise        ──► silent no-op
      │
      ├── ticker.C every interval: repeat the check
      │
      └── SIGINT/SIGTERM
              ▼
        keepawake.Stop() ──► exit 0
```

Guarantee: `keepawake.Stop()` runs on every exit path via `defer`, so `caffeinate` never outlives `antilocker`.

## 8. Error Handling

| Situation | Behavior | Exit code |
|---|---|---|
| Caffeinate missing from PATH | Clear startup error | 1 |
| Config YAML malformed | Parse error + hint to fix the file | 1 |
| `interval` <= 0 / non-numeric | Validation error naming the field (`interval: high` is a YAML type error; `interval: -1` is a validation error) | 1 |
| `networks` key missing **or** explicitly `null`/`~` | Validation error | 1 |
| SSID lookup fails (both commands error) | Timestamped warning, loop continues | — |
| Wi-Fi drops mid-run | Next tick: SSID empty → stop caffeinate, log transition | — |
| `caffeinate` start or stop fails mid-run | Timestamped warning, loop continues | — |
| Ctrl-C / SIGTERM | Stop caffeinate cleanly, exit | 0 |
| Ctrl-C during first-run prompts | `Setup cancelled.`, no partial write | 1 |
| Duplicate SSIDs in config | De-duplicated, startup warning | — |
| Explicit `networks: []` | App runs, matches nothing | — |

## 9. Testing Strategy

Standard Go `testing` package, table-driven tests. No third-party test frameworks.

| Package | Test coverage |
|---|---|
| `internal/config` | Happy-path load; default `interval`; missing `networks` key; malformed YAML; `interval <= 0`; duplicate SSIDs; explicit empty `networks` is legal. Temp-file I/O. |
| `internal/wifi` | Pure parser tests against fixture `ipconfig` and `system_profiler` outputs; empty/not-connected; SSIDs with spaces/quotes/Unicode; fallback ordering. |
| `internal/keepawake` | Fake `exec.Cmd` factory injected: double-start is no-op; stop-when-stopped is no-op; `IsRunning` transitions. Real `caffeinate` invoked only in opt-in integration test. |
| `internal/loop` | Fake `wifi.Provider` (scripted SSID sequence) + fake `keepawake.Manager`: match starts exactly once; no double-start; unmatch stops exactly once; signal triggers stop exactly once. |
| First-run setup | Simulated stdin: interval prompt; networks prompt; empty interval defaults to 3600; invalid interval re-prompts; Ctrl-C cancels without write. |
| Integration | Opt-in real `ipconfig getsummary` test, skipped under `testing.Short()`. |

### Quality gates

- `go test ./...` — green
- `go vet ./...` — clean
- `gofmt -s -l .` — empty
- Build: `GOOS=darwin go build -o antilocker .`

## 10. Out of Scope

- Windows / Linux support
- Screen lock inhibition or display-off inhibition via caffeinate
- Background daemon mode, `start`/`stop`/`status` subcommands
- Event-driven Wi-Fi change detection (CoreWLAN / cgo)
- PID files, IPC
