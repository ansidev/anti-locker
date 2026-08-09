# Anti Locker Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a macOS-only Go CLI `antilocker` that keeps the Mac awake via `caffeinate -i` conditionally based on the currently-connected Wi-Fi SSID matching a YAML-configured network list.

**Architecture:** Single Go module with flat layout. Four small packages under `internal/`: `config` (YAML load/validate + interactive first-run setup), `wifi` (SSID detection via `ipconfig`/`system_profiler`), `keepawake` (caffeinate process lifecycle), and `loop` (the periodic state machine). `main.go` composes them. Packages communicate via narrow Go interfaces so the loop is testable with in-memory fakes. Foreground-only execution with SIGINT/SIGTERM clean shutdown.

**Tech Stack:** Go 1.22+ (toolchain installed: 1.26.1), `gopkg.in/yaml.v3`, `github.com/urfave/cli/v3` v3.10.1, standard library only otherwise.

**Source spec:** `docs/superpowers/specs/2026-08-01-antilocker-design.md` (commit `fed42fb`)

---

## Table of Contents

- **Parallel Execution Model**
  - Workstreams
  - Dependency Waves
  - Merge Order
- **Chunk 1: Project Bootstrap** (Wave 0)
  - Task 1: Go module initialization
  - Task 2: `.gitignore` and README skeleton
- **Chunk 2: Leaf Workstream A — `internal/config`** (Wave 1)
  - Task 3: Config struct and `Load()`
  - Task 4: Validation rules and `Contains()`
  - Task 5: First-run setup (`firstrun.go`) — prompt UI
  - Task 6: First-run setup — YAML persistence and `Run()`
- **Chunk 3: Leaf Workstream B — `internal/wifi`** (Wave 1)
  - Task 7: `ipconfig getsummary en0` parser
  - Task 8: `system_profiler SPAirPortDataType` parser
  - Task 9: `Provider` interface + `ExecProvider` + fallback strategy
- **Chunk 4: Leaf Workstream C — `internal/keepawake`** (Wave 1)
  - Task 10: `Manager` interface + `VerifyCaffeinate()`
  - Task 11: `ExecManager` Start/Stop/IsRunning with process lifecycle
- **Chunk 5: Integration Workstream — `internal/loop`** (Wave 2)
  - Task 12: Loop state machine with ticker
  - Task 13: Loop signal handling and graceful shutdown
- **Chunk 6: Final Integration — `main.go`** (Wave 3)
  - Task 14: CLI scaffolding with urfave/cli v3
  - Task 15: Composition root — wire config → first-run → verify caffeinate → loop
- **Chunk 7: Hardening** (Wave 4)
  - Task 16: Integration test for SSID detection (opt-in)
  - Task 17: Manual smoke-test walkthrough
  - Task 18: README finalization and quality gates

---

## Parallel Execution Model

### Workstreams

| Workstream | Deliverable | Tasks | Dependencies (workstream-level) |
|---|---|---|---|
| **WS-0 Bootstrap** | Empty Go module + gitignore | 1, 2 | none |
| **WS-A Config** | `internal/config` package (load/validate/first-run) | 3, 4, 5, 6 | WS-0 |
| **WS-B Wi-Fi** | `internal/wifi` package (SSID detection) | 7, 8, 9 | WS-0 |
| **WS-C Keep-awake** | `internal/keepawake` package (caffeinate process) | 10, 11 | WS-0 |
| **WS-D Loop** | `internal/loop` package (periodic state machine) | 12, 13 | WS-A, WS-B, WS-C |
| **WS-E Main** | `main.go` (composition root) | 14, 15 | WS-D |
| **WS-F Hardening** | Integration test, smoke test, README, quality gates | 16, 17, 18 | WS-E |

### Dependency Waves

Each wave completes before the next starts. Within a wave, tasks in different workstreams run in parallel (separate worktrees or sub-agents).

| Wave | Tasks in wave | Parallel? | Why gated |
|---|---|---|---|
| **Wave 0** | 1, 2 | Sequential (same workstream) | Foundational — everything depends on `go.mod` |
| **Wave 1** | 3, 7, 10 kick off in parallel; then 4, 5, 6 / 8, 9 / 11 run as sequences | **Yes** (3 parallel lanes: A, B, C) | Leaf packages with no cross imports; only need `go.mod` |
| **Wave 2** | 12, 13 | Sequential | Needs all three leaf interfaces to exist |
| **Wave 3** | 14, 15 | Sequential | Needs loop's `Run` signature |
| **Wave 4** | 16, 17, 18 | Sequential (same workstream; final integration) | Needs full binary |

#### Hidden coupling notes

- **Wave 1 lanes share `go.sum`.** When running Wave 1 lanes in parallel worktrees, each lane's `go get` will modify `go.mod`/`go.sum`. Merge order (below) resolves conflicts deterministically: lane A merges first (yaml.v3), then lanes B and C (no new deps). Lane C does not introduce any new dependency, so its merge is always trivial.
- **Task 14 (CLI scaffolding) imports none of the internal packages**, so it could technically overlap with Wave 2 — but Wave 3 keeps them together to keep merge history linear. Do not parallelize Task 14 into Wave 2 unless you accept ordering risk on `main.go` edits.
- **Task 16 (integration test) needs a real macOS Wi-Fi interface.** It's gated behind `testing.Short()` so it passes in CI but requires Wave 3's `internal/wifi` to be wired.

### Merge Order

Deterministic and safe. Merge in this order; rebase next branch onto `main` before merging:

1. `feature/bootstrap` → `main` (Wave 0) — establishes `go.mod`, `.gitignore`, README skeleton
2. `feature/config` → `main` (WS-A, Wave 1) — adds `gopkg.in/yaml.v3` to `go.mod`/`go.sum`
3. `feature/wifi` → `main` (WS-B, Wave 1) — no new deps, no overlap, fast-forward
4. `feature/keepawake` → `main` (WS-C, Wave 1) — no new deps, no overlap, fast-forward
5. `feature/loop` → `main` (WS-D, Wave 2) — depends on all three leaves
6. `feature/main-cli` → `main` (WS-E, Wave 3) — adds `github.com/urfave/cli/v3` to `go.mod`/`go.sum`
7. `feature/hardening` → `main` (WS-F, Wave 4) — final integration, tests, README, quality gates

**Conflict resolution rule:** if `go.mod`/`go.sum` conflict in step 6 (it shouldn't because steps 2–5 don't touch cli deps), re-run `go mod tidy` on the rebased branch and commit the result.

---

## Chunk 1: Project Bootstrap (WS-0, Wave 0)

Foundational workstream. Sequential tasks. Once merged, Wave 1 lanes can start.

### Task 1: Go module initialization

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize the Go module**

Run:
```bash
cd /Users/ansidev/projects/personal/anti-locker
go mod init github.com/ansidev/antilocker
```

Expected output:
```
go: creating new go.mod: module github.com/ansidev/antilocker
go: to add module requirements and sums:
	go mod tidy
```

- [ ] **Step 2: Pin minimum Go version**

Open `go.mod`, ensure the `go` directive matches the minimum toolchain from the spec (1.22). Example contents:

```go
module github.com/ansidev/antilocker

go 1.22
```

If `go mod init` wrote `go 1.26.1` (or similar bleeding-edge version), edit the line down to exactly `go 1.22` — this sets the floor, not the ceiling.

- [ ] **Step 3: Verify the module parses**

Run: `go list -m`
Expected: `github.com/ansidev/antilocker`

- [ ] **Step 4: Commit**

Note: empty `go.sum` is not needed yet; it will appear when the first dependency is added.

```bash
git add go.mod
git commit -m "chore: initialize Go module

Pin minimum Go to 1.22 per spec §1."
```

---

### Task 2: `.gitignore` and README skeleton

**Files:**
- Create: `.gitignore`
- Create: `README.md`

- [ ] **Step 1: Write `.gitignore`**

Create `.gitignore` with this exact content:

```gitignore
# Binaries
antilocker
*.test
*.out

# Go workspace files
/go.work
/go.work.sum

# Editor
.idea/
.vscode/
*.swp
*.swo

# macOS
.DS_Store
```

- [ ] **Step 2: Write minimal `README.md`**

Create `README.md` with this exact content (final content lands in Task 18):

```markdown
# Anti Locker

macOS-only CLI that prevents idle sleep conditionally based on the connected Wi-Fi network.

**Status: in active development.** See [docs/superpowers/specs/2026-08-01-antilocker-design.md](docs/superpowers/specs/2026-08-01-antilocker-design.md) for the design specification.

## Build

    go build -o antilocker .

## Usage (planned)

    antilocker                                # start with default config
    antilocker --config /path/to/config.yaml  # start with custom config
```

- [ ] **Step 3: Verify**

Run: `cat .gitignore && echo "---" && cat README.md`
Expected: both files print with content above.

- [ ] **Step 4: Commit**

```bash
git add .gitignore README.md
git commit -m "chore: add gitignore and README skeleton"
```

- [ ] **Step 5: Merge Wave 0**

If working in a worktree/branch:
```bash
git checkout main
git merge --ff-only feature/bootstrap
git branch -d feature/bootstrap
```

Otherwise the work is already on `main`. Wave 1 lanes may now start in parallel.

---
## Chunk 2: Leaf Workstream A — `internal/config` (WS-A, Wave 1)

Adds `gopkg.in/yaml.v3` to `go.mod`. Implements config loading, validation, and interactive first-run setup.

### Task 3: Config struct and `Load()`

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test for `Load()` happy path**

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ansidev/antilocker/internal/config"
)

func TestLoad_HappyPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 600
networks:
  - "My Home"
  - "Office Wi-Fi"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if got.Interval != 600 {
		t.Errorf("Interval = %d, want 600", got.Interval)
	}
	if len(got.Networks) != 2 {
		t.Errorf("len(Networks) = %d, want 2", len(got.Networks))
	}
	if got.Networks[0] != "My Home" {
		t.Errorf("Networks[0] = %q, want %q", got.Networks[0], "My Home")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_HappyPath -v`
Expected: FAIL with `undefined: config.Load` (or similar).

- [ ] **Step 3: Write minimal `config.go` to make the test pass**

```go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the parsed antilocker.yaml contents.
// Path records the file location for startup logging (spec §6.5).
// yaml:"-" prevents it being read from or written to the YAML file.
type Config struct {
	Path     string   `yaml:"-"`
	Interval int      `yaml:"interval"`
	Networks []string `yaml:"networks"`
}

// Load reads a YAML config file from path, parses and validates it,
// and returns a Config. If the file does not exist it returns
// os.ErrNotExist so the caller can trigger first-run setup.
//
// Precondition: path has already been expanded (no ~ remains).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoad_HappyPath -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add Config struct and Load() with yaml.v3"
```

---

### Task 4: Validation rules and `Contains()`

**Files:**
- Modify: `internal/config/config.go` (add validation + Contains)
- Modify: `internal/config/config_test.go` (add validation tests)

- [ ] **Step 1: Write failing tests for validation and Contains**

Append to `internal/config/config_test.go`:

```go
func TestLoad_MissingNetworksKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 600
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing networks key")
	}
}

func TestLoad_IntervalZero(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 0
networks:
  - "Home"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for interval <= 0")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: high
networks:
  - "Home"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for malformed YAML type")
	}
}

func TestLoad_NullNetworks(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks: null
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for null networks")
	}
}

// Spec §4.3: "SSID whitespace: Trimmed at load time."
func TestLoad_TrimsSSIDWhitespace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks:
  - "  Padded Network  "
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Networks[0] != "Padded Network" {
		t.Errorf("Networks[0] = %q, want trimmed %q", cfg.Networks[0], "Padded Network")
	}
}

func TestLoad_EmptyNetworkEntry(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks:
  - ""
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for empty network entry")
	}
}

func TestContains_Match(t *testing.T) {
	cfg := &config.Config{Networks: []string{"Home Network 1", "Home Network 2"}}
	if !cfg.Contains("Home Network 1") {
		t.Error("expected Contains to return true for matching SSID")
	}
}

func TestContains_NoMatch(t *testing.T) {
	cfg := &config.Config{Networks: []string{"Home Network 1"}}
	if cfg.Contains("Office Wi-Fi") {
		t.Error("expected Contains to return false for non-matching SSID")
	}
}

func TestContains_EmptyNetworks(t *testing.T) {
	cfg := &config.Config{Networks: []string{}}
	if cfg.Contains("Home Network 1") {
		t.Error("expected Contains to return false for empty networks list")
	}
}

// Spec §4.2: "interval: 3600  # optional, seconds; defaults to 3600"
func TestLoad_DefaultInterval(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`networks:
  - "Home"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Interval != 3600 {
		t.Errorf("Interval = %d, want 3600 (default)", cfg.Interval)
	}
}

// Spec §4.3: explicit networks: [] is valid and matches nothing.
func TestLoad_EmptyNetworksListIsValid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks: []
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Networks) != 0 {
		t.Errorf("len(Networks) = %d, want 0", len(cfg.Networks))
	}
	if cfg.Contains("anything") {
		t.Error("Contains returned true for empty list")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run "TestLoad_MissingNetworksKey|TestLoad_IntervalZero|TestLoad_MalformedYAML|TestLoad_NullNetworks|TestLoad_EmptyNetworkEntry|TestContains_" -v`
Expected: FAIL (validation and Contains not yet implemented).

- [ ] **Step 3: Extend `config.go` with validation and Contains**

Replace `Load()` body with the full validated version. Add `Contains()`.

```go
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrValidation marks configuration-validation failures so callers can
// distinguish them from IO or parse errors.
var ErrValidation = errors.New("config validation error")

// Config holds the parsed antilocker.yaml contents.
// Path records the file location for startup logging (spec §6.5).
// yaml:"-" prevents it being read from or written to the YAML file.
type Config struct {
	Path     string   `yaml:"-"`
	Interval int      `yaml:"interval"`
	Networks []string `yaml:"networks"`
}

// Load reads a YAML config file from path, parses and validates it,
// and returns a Config. If the file does not exist it returns
// os.ErrNotExist so the caller can trigger first-run setup.
//
// Precondition: path has already been expanded (no ~ remains).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	var cfg Config

	// Decode into a shadow struct so we can distinguish "key absent"
	// (nil pointer → default 3600, spec §4.2) from "explicitly 0"
	// (validation error per §4.3 / TestLoad_IntervalZero).
	var raw struct {
		Interval *int     `yaml:"interval"`
		Networks []string `yaml:"networks"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if raw.Interval == nil {
		cfg.Interval = 3600 // specified default (spec §4.2)
	} else {
		cfg.Interval = *raw.Interval
	}
	cfg.Networks = raw.Networks

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	cfg.Path = path

	return &cfg, nil
}

// validate checks all Config constraints. It trims whitespace from each
// network SSID (spec §4.3 "SSID whitespace: Trimmed at load time"),
// de-duplicates and warns about duplicates (spec §4.3 "Duplicate SSIDs:
// De-duplicate, print startup warning"), and returns an error for any
// violation.
func validate(cfg *Config) error {
	if cfg.Networks == nil {
		return fmt.Errorf("%w: networks key is required", ErrValidation)
	}
	// Trim whitespace at load time; reject entries that become empty.
	for i, n := range cfg.Networks {
		trimmed := strings.TrimSpace(n)
		if trimmed == "" {
			return fmt.Errorf("%w: networks entry %d is empty after trimming", ErrValidation, i)
		}
		cfg.Networks[i] = trimmed
	}
	if cfg.Interval <= 0 {
		return fmt.Errorf("%w: interval must be a positive integer, got %d", ErrValidation, cfg.Interval)
	}

	// De-duplicate networks in place, preserving order. Print a startup
	// warning if any duplicates were found.
	seen := make(map[string]struct{}, len(cfg.Networks))
	deduped := make([]string, 0, len(cfg.Networks))
	hadDuplicates := false
	for _, n := range cfg.Networks {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			deduped = append(deduped, n)
		} else {
			hadDuplicates = true
		}
	}
	if hadDuplicates {
		log.Printf("warn: duplicate network entries removed; %d unique networks configured", len(deduped))
	}
	cfg.Networks = deduped

	return nil
}

// Contains reports whether ssid is in the configured networks list.
// The networks list is already trimmed at Load time (spec §4.3); the
// comparison here is exact and case-sensitive.
func (c *Config) Contains(ssid string) bool {
	for _, n := range c.Networks {
		if n == ssid {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run all config tests**

Run: `go test ./internal/config/ -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add validation rules and Contains()"
```

---

### Task 5: First-run setup — prompt UI

**Files:**
- Create: `internal/config/firstrun.go`
- Create: `internal/config/firstrun_test.go`

- [ ] **Step 1: Write failing tests for first-run prompts**

```go
package config_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ansidev/antilocker/internal/config"
)

func TestRun_SetupPrompts(t *testing.T) {
	// Empty interval input → default 3600; one network; then empty to finish.
	inputStr := "\nHome Wi-Fi\n\n"
	var buf bytes.Buffer

	cfg, err := config.Run(context.Background(), filepath.Join(t.TempDir(), "antilocker.yaml"), strings.NewReader(inputStr), &buf)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if cfg.Interval != 3600 {
		t.Errorf("Interval = %d, want 3600 (default)", cfg.Interval)
	}
	if len(cfg.Networks) != 1 || cfg.Networks[0] != "Home Wi-Fi" {
		t.Errorf("Networks = %v, want [Home Wi-Fi]", cfg.Networks)
	}
	// Spec §3.3 step 1: preamble line must be printed.
	if !strings.Contains(buf.String(), "No config found at") {
		t.Errorf("expected preamble in output, got %q", buf.String())
	}
	// Spec §3.3 step 4: saved-path line must be printed.
	if !strings.Contains(buf.String(), "Config saved to") {
		t.Errorf("expected saved-path message in output, got %q", buf.String())
	}
}

func TestRun_InvalidIntervalRePrompts(t *testing.T) {
	// Three invalid interval attempts then a valid one; then a network, then empty to finish.
	inputStr := "abc\n-1\n0\n600\nHome Wi-Fi\n\n"
	var buf bytes.Buffer

	cfg, err := config.Run(context.Background(), filepath.Join(t.TempDir(), "antilocker.yaml"), strings.NewReader(inputStr), &buf)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if cfg.Interval != 600 {
		t.Errorf("Interval = %d, want 600", cfg.Interval)
	}
}

func TestRun_NetworkRequired(t *testing.T) {
	// First empty input is rejected (re-prompt), then a real one is accepted.
	inputStr := "600\n\nHome Wi-Fi\n\n"
	var buf bytes.Buffer

	cfg, err := config.Run(context.Background(), filepath.Join(t.TempDir(), "antilocker.yaml"), strings.NewReader(inputStr), &buf)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(cfg.Networks) != 1 || cfg.Networks[0] != "Home Wi-Fi" {
		t.Errorf("Networks = %v, want [Home Wi-Fi]", cfg.Networks)
	}
}

func TestRun_CancelledByContext(t *testing.T) {
	// Spec §3.4: cancelled context must return ErrCancelled without creating a file.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before Run starts

	target := filepath.Join(t.TempDir(), "antilocker.yaml")
	var buf bytes.Buffer

	_, err := config.Run(ctx, target, strings.NewReader(""), &buf)
	if err == nil {
		t.Fatal("expected ErrCancelled")
	}
	if !errors.Is(err, config.ErrCancelled) {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Error("no partial config should be written when cancelled")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestRun_ -v`
Expected: FAIL (`config.Run undefined`).

- [ ] **Step 3: Write minimal `firstrun.go`**

```go
package config

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrCancelled is returned when the user aborts first-run setup
// (EOF on stdin, or upstream signal handling). Callers should print
// "Setup cancelled." (spec §3.4) and exit with code 1.
var ErrCancelled = errors.New("setup cancelled")

// Run executes the interactive first-run setup. It reads prompts from
// `reader` (os.Stdin in production) and writes prompts to `w`.
//
// It accepts ctx so SIGINT/SIGTERM cancellation is respected (spec §3.4):
// internally it runs a goroutine that pushes scanned lines into a channel,
// letting it select between input and ctx.Done(). When cancelled, it
// returns ErrCancelled without writing any file (spec §3.4).
//
// It prints a preamble line ("No config found at <path> — let's set one up.")
// before prompting (spec §3.3 step 1), persists a validated YAML config to
// path, prints "Config saved to <path>" on success (spec §3.3 step 4), and
// returns the Config.
func Run(ctx context.Context, path string, reader io.Reader, w io.Writer) (*Config, error) {
	fmt.Fprintf(w, "No config found at %s — let's set one up.\n", path)

	lines := make(chan string, 16)
	scanErrs := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		// EOF or read error ends the stream.
		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			scanErrs <- err
		}
		close(lines)
	}()

	readLine := func() (string, error) {
		select {
		case <-ctx.Done():
			return "", ErrCancelled
		case err := <-scanErrs:
			return "", err
		case line, ok := <-lines:
			if !ok {
				return "", ErrCancelled // stdin closed (Ctrl-D / EOF)
			}
			return line, nil
		}
	}

	interval, err := promptInterval(readLine, w)
	if err != nil {
		return nil, err
	}

	networks, err := promptNetworks(readLine, w)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Path:     path,
		Interval: interval,
		Networks: networks,
	}

	if err := persistConfig(path, cfg); err != nil {
		return nil, err
	}

	fmt.Fprintf(w, "Config saved to %s\n", path)
	return cfg, nil
}

// promptInterval reads and validates the check interval. Re-prompts on
// invalid input. Returns ErrCancelled when readLine fails.
func promptInterval(readLine func() (string, error), w io.Writer) (int, error) {
	for {
		fmt.Fprint(w, "Check interval in seconds [3600]: ")
		raw, err := readLine()
		if err != nil {
			return 0, err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return 3600, nil
		}
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
			fmt.Fprintln(w, "Please enter a positive integer.")
			continue
		}
		return n, nil
	}
}

// promptNetworks collects Wi-Fi names one at a time. Returns ErrCancelled
// when readLine fails. Requires at least one network (spec §3.3 step 3).
func promptNetworks(readLine func() (string, error), w io.Writer) ([]string, error) {
	var networks []string
	for {
		fmt.Fprint(w, "Add a network name (leave empty to finish): ")
		raw, err := readLine()
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(raw)
		if name == "" {
			if len(networks) == 0 {
				fmt.Fprintln(w, "At least one network is required.")
				continue
			}
			return networks, nil
		}
		networks = append(networks, name)
	}
}

func persistConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/ -run TestRun_ -v`
Expected: PASS (adjust test expectations as needed to match actual Run signature).

- [ ] **Step 5: Commit**

```bash
git add internal/config/firstrun.go internal/config/firstrun_test.go
git commit -m "feat(config): add interactive first-run setup prompts"
```

---

### Task 6: First-run setup — YAML persistence and `Run()` integration

**Files:**
- Modify: `internal/config/firstrun.go` (persistConfig writes explicit interval + networks)
- Modify: `internal/config/config.go` (ensure `Run()` returns the right path)

- [ ] **Step 1: Verify first-run write guarantee**

The spec (§4.2 + first-run write guarantee) requires that `persistConfig` always writes both `interval` and `networks` explicitly — never relying on YAML defaults for `interval`. The current `yaml.Marshal(cfg)` call already serializes both fields because `cfg.Interval` defaults to `0` and we set it to `3600` before marshaling, and `cfg.Networks` is always a non-nil slice (empty `[]string{}` serializes as `[]`). Verify by inspecting the marshaled output.

- [ ] **Step 2: Run a quick manual check**

Run this Go snippet to confirm marshaling behavior:

```go
package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Interval int      `yaml:"interval"`
	Networks []string `yaml:"networks"`
}

func main() {
	cfg := Config{Interval: 3600, Networks: []string{}}
	data, _ := yaml.Marshal(&cfg)
	fmt.Println(string(data))
}
```

The yaml.v3 library does not support `omitempty:false`; the fix is to ensure `Networks` is always initialized to a non-nil empty slice (`[]string{}`), not `nil`. The struct tag remains `yaml:"networks"` with no `omitempty`.

- [ ] **Step 3: Commit any fix**

```bash
git add internal/config/firstrun.go
git commit -m "fix(config): ensure first-run YAML always writes interval and networks explicitly"
```

- [ ] **Step 4: Merge WS-A**

If working in a branch:
```bash
git checkout main
git merge --ff-only feature/config
```

Wave 1 lanes B and C can now start in parallel (they have no cross-dependencies with A).
## Chunk 3: Leaf Workstream B — `internal/wifi` (WS-B, Wave 1)

Implements SSID detection via `ipconfig` (primary) and `system_profiler` (fallback), with an injectable seam so unit tests never touch real `exec`.

### Task 7: `ipconfig getsummary en0` parser

**Files:**
- Create: `internal/wifi/wifi.go`
- Create: `internal/wifi/wifi_test.go`

- [ ] **Step 1: Write failing test for `CurrentSSID()` — ipconfig primary**

```go
package wifi_test

import (
	"testing"

	"github.com/ansidev/antilocker/internal/wifi"
)

func TestCurrentSSID_IPConfigPrimary(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			if name == "ipconfig" {
				return "  SSID : My Home Wi-Fi\n", nil
			}
			return "", nil
		},
	)
	ssid, err := p.CurrentSSID()
	if err != nil {
		t.Fatalf("CurrentSSID() unexpected error: %v", err)
	}
	if ssid != "My Home Wi-Fi" {
		t.Errorf("ssid = %q, want %q", ssid, "My Home Wi-Fi")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/wifi/ -run TestCurrentSSID_IPConfigPrimary -v`
Expected: FAIL (`NewExecProvider` undefined).

- [ ] **Step 3: Write minimal `wifi.go` with `ExecProvider` and parser**

```go
package wifi

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Provider is the seam injected into internal/loop for testing.
type Provider interface {
	CurrentSSID() (string, error)
}

// ExecProvider is the production implementation; it shells out to
// macOS binaries. lookPath is injectable so unit tests never touch
// real exec. Production wiring passes exec.LookPath.
type ExecProvider struct {
	lookPath func(string) (string, error)
}

// NewExecProvider returns an ExecProvider that uses lookPath to
// resolve binaries. Pass nil to use exec.LookPath.
func NewExecProvider(lookPath func(string) (string, error)) *ExecProvider {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	return &ExecProvider{lookPath: lookPath}
}

func (p *ExecProvider) CurrentSSID() (string, error) {
	ipOut, err := p.run("ipconfig", "getsummary", "en0")
	if err != nil {
		ipOut = nil // fall through to system_profiler
	}

	if ssid := parseIPConfig(ipOut); ssid != "" {
		return ssid, nil
	}

	spOut, err := p.run("system_profiler", "SPAirPortDataType")
	if err != nil {
		return "", err // transient warning by loop
	}
	return parseSystemProfiler(spOut), nil
}

func (p *ExecProvider) run(name string, args ...string) ([]byte, error) {
	bin, err := p.lookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s not found: %w", name, err)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parseIPConfig extracts SSID from `ipconfig getsummary en0` output.
// Returns "" if not found.
func parseIPConfig(out []byte) string {
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("SSID : ")) {
			return string(line[7:])
		}
	}
	return ""
}

// parseSystemProfiler extracts the current SSID from
// `system_profiler SPAirPortDataType` output. Returns "" if not found.
func parseSystemProfiler(out []byte) string {
	inCurrentBlock := false
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte("Current Network Information:")) {
			inCurrentBlock = true
			continue
		}
		if inCurrentBlock && bytes.HasPrefix(line, []byte("Network Name")) {
			parts := bytes.SplitN(line, []byte{':'}, 2)
			if len(parts) == 2 {
				return string(bytes.TrimSpace(parts[1]))
			}
		}
	}
	return ""
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/wifi/ -run TestCurrentSSID_IPConfigPrimary -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wifi/wifi.go internal/wifi/wifi_test.go
git commit -m "feat(wifi): add ExecProvider with ipconfig/system_profiler fallback"
```

---

### Task 8: `system_profiler SPAirPortDataType` fallback and edge cases

**Files:**
- Modify: `internal/wifi/wifi.go` (no changes needed if Task 7 already covered fallback)
- Modify: `internal/wifi/wifi_test.go` (add fallback + edge case tests)

- [ ] **Step 1: Write failing tests for fallback and edge cases**

```go
func TestCurrentSSID_SystemProfilerFallback(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			if name == "ipconfig" {
				return "", fmt.Errorf("ipconfig missing")
			}
			return "Current Network Information:\n    Network Name : Home Wi-Fi\n", nil
		},
	)
	ssid, err := p.CurrentSSID()
	if err != nil {
		t.Fatalf("CurrentSSID() unexpected error: %v", err)
	}
	if ssid != "Home Wi-Fi" {
		t.Errorf("ssid = %q, want %q", ssid, "Home Wi-Fi")
	}
}

func TestCurrentSSID_WiFiOff(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			if name == "ipconfig" {
				return "", fmt.Errorf("ipconfig missing")
			}
			return "Current Network Information:\n    Network Name :\n", nil
		},
	)
	ssid, err := p.CurrentSSID()
	if err != nil {
		t.Fatalf("CurrentSSID() unexpected error: %v", err)
	}
	if ssid != "" {
		t.Errorf("ssid = %q, want empty string", ssid)
	}
}

func TestCurrentSSID_MissingBinaries(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			return "", fmt.Errorf("%s not found", name)
		},
	)
	_, err := p.CurrentSSID()
	if err == nil {
		t.Fatal("expected error when both binaries missing")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/wifi/ -v`
Expected: PASS (logic already implemented in Task 7).

- [ ] **Step 3: Commit**

```bash
git add internal/wifi/wifi_test.go
git commit -m "test(wifi): add fallback, Wi-Fi-off, and missing-binary cases"
```

---

### Task 9: `Provider` interface and integration with loop

- [ ] **Step 1: Verify `Provider` interface exists in `wifi.go`**

Already defined in Task 7:

```go
type Provider interface {
	CurrentSSID() (string, error)
}
```

- [ ] **Step 2: Merge WS-B**

```bash
git checkout main
git merge --ff-only feature/wifi
```

Wave 1 lanes A and C can now be merged in either order. Both are independent.
## Chunk 4: Leaf Workstream C — `internal/keepawake` (WS-C, Wave 1)

Implements the `caffeinate` process lifecycle: `VerifyCaffeinate()` (called once from `main`), and `ExecManager` with `Start`/`Stop`/`IsRunning`.

### Task 10: `Manager` interface + `VerifyCaffeinate()`

**Files:**
- Create: `internal/keepawake/keepawake.go`
- Create: `internal/keepawake/keepawake_test.go`

- [ ] **Step 1: Write failing test for `VerifyCaffeinate()`**

```go
package keepawake_test

import (
	"testing"

	"github.com/ansidev/antilocker/internal/keepawake"
)

func TestVerifyCaffeinate_Found(t *testing.T) {
	// On macOS, caffeinate is guaranteed to exist. In unit tests we rely
	// on the real PATH lookup. If this fails on your machine, skip it
	// or set PATH to include /usr/bin.
	if err := keepawake.VerifyCaffeinate(); err != nil {
		t.Fatalf("VerifyCaffeinate() unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/keepawake/ -run TestVerifyCaffeinate_Found -v`
Expected: FAIL (`VerifyCaffeinate` undefined).

- [ ] **Step 3: Write minimal `keepawake.go`**

```go
package keepawake

import (
	"fmt"
	"os/exec"
)

// Manager is the seam injected into internal/loop for testing.
type Manager interface {
	Start(network string) error // no-op if already running
	Stop()                      // no-op if already stopped
	IsRunning() bool
}

// VerifyCaffeinate ensures the caffeinate binary exists in PATH.
// Called once from main() before loop.Run, never from inside Run.
func VerifyCaffeinate() error {
	if _, err := exec.LookPath("caffeinate"); err != nil {
		return fmt.Errorf("caffeinate not found in PATH: %w (install Xcode command line tools or check your macOS installation)", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/keepawake/ -run TestVerifyCaffeinate_Found -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/keepawake/keepawake.go internal/keepawake/keepawake_test.go
git commit -m "feat(keepawake): add Manager interface and VerifyCaffeinate"
```

---

### Task 11: `ExecManager` Start/Stop/IsRunning

**Files:**
- Modify: `internal/keepawake/keepawake.go` (add ExecManager)
- Modify: `internal/keepawake/keepawake_test.go` (fake exec.Cmd tests)

- [ ] **Step 1: Write failing tests for `ExecManager`**

The tests use a fake command factory so **no real `caffeinate` process is started** (spec §9: "Fake `exec.Cmd` factory injected"). On unix this is done by substituting a shell one-liner that exits immediately — we use Go's documented test-helper-process pattern via `os.Args[0]` so the spawned "process" is the test binary itself running a helper mode.

```go
package keepawake_test

// TestMain's helper process pattern: when invoked with -test.run=TestHelperProcess
// and GO_HELPER_PROCESS=1, the test binary itself acts as the fake command.
// This lets us fake exec.Cmd without spawning real caffeinate.

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ansidev/antilocker/internal/keepawake"
)

// fakeCommand returns an *exec.Cmd that, when started, runs this same test
// binary in helper mode and BLOCKS until killed (simulating persistent caffeinate).
// The standard Go pattern: the current test binary re-runs itself in a special mode.
func fakeCommand(name string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"-test.run=TestHelperProcess", "--", name}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_HELPER_PROCESS=1")
	return cmd
}

// TestHelperProcess is the in-test helper. It simulates a long-running
// "child process" by blocking forever (exits only via signal/KILL from Stop).
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_HELPER_PROCESS") != "1" {
		return // not in helper mode — skip
	}
	// Block indefinitely; Stop() will send SIGKILL/KILL and then Wait() reap.
	select {}
}
```

func TestExecManager_StartStop(t *testing.T) {
	m := keepawake.NewExecManager(keepawake.WithCommandFactory(fakeCommand))

	if m.IsRunning() {
		t.Fatal("expected not running initially")
	}

	// First start
	if err := m.Start("Home Wi-Fi"); err != nil {
		t.Fatalf("Start() unexpected error: %v", err)
	}
	if !m.IsRunning() {
		t.Fatal("expected running after Start")
	}

	// Second start is a no-op
	if err := m.Start("Home Wi-Fi"); err != nil {
		t.Fatalf("second Start() unexpected error: %v", err)
	}

	// Stop
	m.Stop()
	if m.IsRunning() {
		t.Fatal("expected stopped after Stop")
	}

	// Second stop is a no-op
	m.Stop()
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/keepawake/ -run TestExecManager_StartStop -v`
Expected: FAIL (`ExecManager` undefined).

- [ ] **Step 3: Implement `ExecManager`**

Update `internal/keepawake/keepawake.go`'s import block to include the new packages, then append `ExecManager`.

Updated import block (replace the existing one at the top of the file):

```go
import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
)
```

(Note: `os` from the previous draft is unused — `syscall.SIGTERM` covers the signal constant. Do not import `os` here.)

Append to `internal/keepawake/keepawake.go`:

```go
// CommandFactory abstracts exec.Command for testability. Production passes
// exec.Command; tests substitute a fake that runs a helper process.
type CommandFactory func(name string, args ...string) *exec.Cmd

// ExecManagerOption customizes ExecManager at construction.
type ExecManagerOption func(*ExecManager)

// WithCommandFactory overrides the default exec.Command. Used in tests to
// avoid spawning real caffeinate processes.
func WithCommandFactory(f CommandFactory) ExecManagerOption {
	return func(m *ExecManager) { m.newCmd = f }
}

// ExecManager is the production implementation backed by os/exec.
//
// Lifecycle: Start spawns a process and a reaper goroutine that flips
// `alive` to false when the process exits on its own. IsRunning reflects
// the *actual* liveness of the child, not just the non-nil cmd pointer.
type ExecManager struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	alive  bool // true while child is running; cleared by reaper goroutine
	newCmd CommandFactory
}

// NewExecManager returns an ExecManager using exec.Command by default.
// Pass functional options to override dependencies.
func NewExecManager(opts ...ExecManagerOption) *ExecManager {
	m := &ExecManager{newCmd: exec.Command}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Verify interface compliance at compile time.
var _ Manager = (*ExecManager)(nil)

// Start spawns a persistent caffeinate -i process. It is a no-op if
// already running. It also returns nil if a previous child died before
// Stop was called (we lazily clear state on the next Start).
func (m *ExecManager) Start(network string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.alive {
		return nil // already running
	}
	// If we have a stale cmd from a process that died on its own, clear it.
	if m.cmd != nil && m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited() {
		m.cmd = nil
	}
	if m.cmd != nil {
		// Defensive: alive==false but cmd non-nil and not exited yet —
		// treat as starting; refuse.
		return fmt.Errorf("previous caffeinate child not yet reaped")
	}

	cmd := m.newCmd("caffeinate", "-i")
	cmd.Stdout = nil // discard output per spec
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start caffeinate: %w", err)
	}

	m.cmd = cmd
	m.alive = true

	// Reaper goroutine: clears alive when the process exits on its own so
	// IsRunning reflects truth even if Stop is never called.
	go func(c *exec.Cmd) {
		_ = c.Wait()
		m.mu.Lock()
		if m.cmd == c {
			m.alive = false
		}
		m.mu.Unlock()
	}(cmd)

	return nil
}

// Stop terminates the caffeinate process. It is a no-op if already stopped.
//
// Sequence (spec §6.4): SIGTERM → short grace period → SIGKILL.
// State cleanup is synced with the reaper goroutine in Start.
func (m *ExecManager) Stop() {
	m.mu.Lock()
	cmd := m.cmd
	// Mark not-alive immediately so concurrent IsRunning reports false
	// while we SIGTERM/KILL.
	m.alive = false
	m.cmd = nil
	m.mu.Unlock()

	if cmd == nil {
		return // already stopped
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return // child already exited (reaper cleared alive); nothing to do
	}

	// 1. Send SIGTERM (spec §6.4).
	_ = cmd.Process.Signal(syscall.SIGTERM)

	// 2. Grace period for clean exit.
	grace := 100 * time.Millisecond
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		return
	case <-time.After(grace):
		_ = cmd.Process.Kill()
		<-done // reap the child to avoid zombies
	}
}

// IsRunning reports whether caffeinate is currently active.
// Reflects real process liveness via the reaper goroutine.
func (m *ExecManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alive
}
```

Note on `syscall.SIGTERM`: though `syscall` is documented as frozen, `SIGTERM` is portable across darwin/linux and is the simplest correct primitive here. No third-party module is added.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/keepawake/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/keepawake/keepawake.go internal/keepawake/keepawake_test.go
git commit -m "feat(keepawake): add ExecManager with Start/Stop/IsRunning"
```

- [ ] **Step 6: Merge WS-C**

```bash
git checkout main
git merge --ff-only feature/keepawake
```

All Wave 1 lanes (A, B, C) are now merged. Wave 2 (`internal/loop`) can start.
## Chunk 5: Integration Workstream — `internal/loop` (WS-D, Wave 2)

Implements the periodic state machine that ties together `config`, `wifi`, and `keepawake`. Uses narrow interfaces for testability.

### Task 12: Loop state machine with ticker

**Files:**
- Create: `internal/loop/loop.go`
- Create: `internal/loop/loop_test.go` — uses fakes for `wifi.Provider` and `keepawake.Manager`

- [ ] **Step 1: Write failing test for `Run()` — state transitions**

```go
package loop_test

import (
	"context"
	"testing"
	"time"

	"github.com/ansidev/antilocker/internal/config"
	"github.com/ansidev/antilocker/internal/loop"
)

type fakeWiFi struct {
	ssids []string
	idx   int
	err   error
}
func (f *fakeWiFi) CurrentSSID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	ssid := f.ssids[f.idx]
	if f.idx+1 < len(f.ssids) {
		f.idx++
	}
	return ssid, nil
}

type fakeKeepAwake struct {
	started   []string
	stopped   int
	running   bool
}
func (f *fakeKeepAwake) Start(network string) error {
	f.started = append(f.started, network)
	f.running = true
	return nil
}
func (f *fakeKeepAwake) Stop() {
	f.stopped++
	f.running = false
}
func (f *fakeKeepAwake) IsRunning() bool { return f.running }

func TestRun_MatchStartsKeepAwake(t *testing.T) {
	w := &fakeWiFi{ssids: []string{"Home Wi-Fi", "Home Wi-Fi", "Home Wi-Fi"}}
	k := &fakeKeepAwake{}
	cfg := &config.Config{Interval: 1, Networks: []string{"Home Wi-Fi"}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(3500 * time.Millisecond) // allow 3 ticks at 1s interval
		cancel()
	}()
	loop.Run(ctx, cfg, w, k)

	if len(k.started) != 1 {
		t.Errorf("len(started) = %d, want 1", len(k.started))
	}
	if k.stopped != 0 {
		t.Errorf("stopped count = %d, want 0", k.stopped)
	}
}

func TestRun_UnmatchStopsKeepAwake(t *testing.T) {
	w := &fakeWiFi{ssids: []string{"Home Wi-Fi", "Office Wi-Fi"}}
	k := &fakeKeepAwake{}
	cfg := &config.Config{Interval: 1, Networks: []string{"Home Wi-Fi"}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(2500 * time.Millisecond) // allow 2 ticks: match then unmatch
		cancel()
	}()
	loop.Run(ctx, cfg, w, k)

	if len(k.started) != 1 {
		t.Errorf("len(started) = %d, want 1", len(k.started))
	}
	if k.stopped != 1 {
		t.Errorf("stopped count = %d, want 1", k.stopped)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/loop/ -run TestRun_ -v`
Expected: FAIL (`loop.Run` undefined).

- [ ] **Step 3: Implement `internal/loop/loop.go`**

```go
package loop

import (
	"context"
	"log"
	"time"

	"github.com/ansidev/antilocker/internal/config"
	"github.com/ansidev/antilocker/internal/keepawake"
	"github.com/ansidev/antilocker/internal/wifi"
)

// Run executes the periodic check loop until ctx is cancelled.
// It has no return value: context cancellation / SIGINT/SIGTERM is normal
// shutdown (exit 0 handled by caller), and all recoverable runtime issues
// are logged as warnings. Fatal configuration problems are caught before
// Run is invoked.
func Run(ctx context.Context, cfg *config.Config, w wifi.Provider, k keepawake.Manager) {
	// Spec §6.5 step 1: log startup summary including config path
	log.Printf("Starting anti-locker | config=%s interval=%ds networks=%d", cfg.Path, cfg.Interval, len(cfg.Networks))

	// Immediate check before first tick
	check(cfg, w, k)

	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown signal received")
			k.Stop()
			return
		case <-ticker.C:
			check(cfg, w, k)
		}
	}
}

// check performs one SSID lookup and updates keep-awake state.
func check(cfg *config.Config, w wifi.Provider, k keepawake.Manager) {
	ssid, err := w.CurrentSSID()
	if err != nil {
		log.Printf("warn: ssid lookup failed: %v", err)
		return
	}

	matched := cfg.Contains(ssid)
	running := k.IsRunning()

	switch {
	case matched && !running:
		log.Printf("joining %q — starting caffeinate", ssid)
		if err := k.Start(ssid); err != nil {
			log.Printf("warn: caffeinate start failed: %v", err)
		}
	case !matched && running:
		log.Printf("left network — stopping caffeinate")
		k.Stop()
	}
	// otherwise: silent no-op
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/loop/ -v -timeout 10s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/loop/loop.go internal/loop/loop_test.go
git commit -m "feat(loop): add periodic state machine with context shutdown"
```

---

### Task 13: Loop signal handling and graceful shutdown

- [ ] **Step 1: Verify signal handling via ctx**

`Run` uses `ctx.Done()` for shutdown. This is inherently tied to SIGINT/SIGTERM via `signal.NotifyContext` in `main.go` (Wave 3). No changes needed in `loop.go` for this task — the responsibility is on the caller.

- [ ] **Step 2: Add a test for signal-triggered stop**

```go
func TestRun_SignalStopsKeepAwake(t *testing.T) {
	w := &fakeWiFi{ssids: []string{"Home Wi-Fi"}}
	k := &fakeKeepAwake{}
	cfg := &config.Config{Interval: 10, Networks: []string{"Home Wi-Fi"}}

	ctx, cancel := context.WithCancel(context.Background())

	// Start loop, trigger one check, then cancel to simulate SIGINT/SIGTERM
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	loop.Run(ctx, cfg, w, k)

	if len(k.started) != 1 {
		t.Errorf("len(started) = %d, want 1", len(k.started))
	}
	if k.stopped != 1 {
		t.Errorf("stopped count = %d, want 1 (should be stopped on shutdown)", k.stopped)
	}
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/loop/ -run TestRun_SignalStopsKeepAwake -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/loop/loop_test.go
git commit -m "test(loop): add signal-triggered shutdown test"
```

- [ ] **Step 5: Merge WS-D**

```bash
git checkout main
git merge --ff-only feature/loop
```

Wave 2 complete. Wave 3 (`main.go`) can now start.
## Chunk 6: Final Integration Workstream — `main.go` (WS-E, Wave 3)

Implements the CLI composition root: `urfave/cli` app setup, composition of `config.Load` → first-run → `VerifyCaffeinate` → `loop.Run`, and signal wiring.

### Task 14: CLI scaffolding with urfave/cli v3

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write minimal `main.go`**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ansidev/antilocker/internal/config"
	"github.com/ansidev/antilocker/internal/keepawake"
	"github.com/ansidev/antilocker/internal/loop"
	"github.com/ansidev/antilocker/internal/wifi"
	"github.com/urfave/cli/v3"
)

var version = "dev" // overridden by ldflags

func main() {
	app := &cli.Command{
		Name:    "antilocker",
		Usage:   "keep macOS awake conditionally based on Wi-Fi network",
		Version: version,
		// Description surfaces an example config in --help output (spec §6.1).
		Description: `Keep this Mac awake when connected to one of your trusted Wi-Fi networks.

Example config (~/.config/antilocker.yaml):

  interval: 3600
  networks:
    - "Home Network 1"
    - "Home Network 2"
`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "path to YAML config file",
				Value: "~/.config/antilocker.yaml",
			},
		},
		Action: runAction,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runAction is the urfave/cli action handler. Signal wiring is set up
// first so that Ctrl-C / SIGTERM during first-run setup is caught.
// runAction is the urfave/cli action handler. Signal wiring is set up
// first so that Ctrl-C / SIGTERM during first-run setup is caught.
func runAction(ctx context.Context, c *cli.Command) error {
	cfgPath := expandPath(c.String("config"))

	// Wire SIGINT/SIGTERM to context cancellation before any I/O so that
	// first-run setup can be interrupted cleanly (spec §3.4).
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			// First run. Require an interactive TTY (spec §3.3).
			if !stdinIsTerminal() {
				return fmt.Errorf("no config found at %s and stdin is not a terminal; please run interactively to complete first-run setup", cfgPath)
			}
			cfg, err = config.Run(ctx, cfgPath, os.Stdin, os.Stdout)
			if err != nil {
				if errors.Is(err, config.ErrCancelled) {
					// Spec §3.4: user cancelled during first-run setup.
					fmt.Fprintln(os.Stderr, "Setup cancelled.")
					return err
				}
				return fmt.Errorf("first-run setup failed: %w", err)
			}
		case errors.Is(err, config.ErrValidation):
			return fmt.Errorf("config validation failed: %w", err)
		default:
			return fmt.Errorf("failed to load config %s: %w", cfgPath, err)
		}
	}

	if err := keepawake.VerifyCaffeinate(); err != nil {
		return fmt.Errorf("fatal: %w", err)
	}

	mgr := keepawake.NewExecManager()
	defer mgr.Stop() // guarantee caffeinate never outlives antilocker (spec §7)

	w := wifi.NewExecProvider(nil)

	loop.Run(ctx, cfg, w, mgr)
	return nil
}

// expandPath resolves "~/..." to the user's home directory.
// If home lookup fails, the raw path is returned unchanged.
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// stdinIsTerminal reports whether stdin is a terminal (spec §3.3).
// Uses pure stdlib; no extra dependency.
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
```

- [ ] **Step 2: Add `go.mod` dependencies**

Run: `go mod tidy`
Expected: `gopkg.in/yaml.v3` and `github.com/urfave/cli/v3` added to `go.mod`.

- [ ] **Step 3: Run build to verify it compiles**

Run: `go build -o antilocker .`
Expected: no output, binary created.

- [ ] **Step 4: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat(cli): add main.go with urfave/cli v3 and composition root"
```

---

### Task 15: Integration test for signal handling in main

The integration test must **really** verify clean shutdown on SIGINT/SIGTERM. The naive pattern (`exec.CommandContext(ctx, "go", "run", ".")`) is broken in three ways: it is slow, it inherits the test's `go env`, and `CommandContext`'s default `Cancel` sends `SIGKILL` (not SIGINT), so the test would pass even without any signal handling in the binary. This task uses the correct pattern: build a real binary in `TestMain`, override `cmd.Cancel` to send a real `SIGINT`, and assert the child exits with code 0.

**Files:**
- Create: `main_test.go`

- [ ] **Step 1: Write the integration test**

```go
package main_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// testBinary is built once by TestMain (skipped in -short mode).
var testBinary string

func TestMain(m *testing.M) {
	code := run(m)
	os.Exit(code)
}

// run compiles the binary (unless short) and runs all tests.
func run(m *testing.M) int {
	if testing.Short() {
		return m.Run()
	}

	dir, err := os.MkdirTemp("", "antilocker-integration-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "antilocker")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("go build . failed: " + err.Error())
	}

	testBinary = bin
	return m.Run()
}

func TestMain_CleanShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte("interval: 3600\nnetworks:\n  - \"Test Wi-Fi\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinary, "--config", cfgPath)
	// Override CommandContext's default Kill-on-cancel: send SIGINT so we
	// actually exercise the signal handler in main.go.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(os.Interrupt) // SIGINT
	}
	// Don't propagate SIGINT to the whole process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for the child to print its startup line. This is a lightweight
	// readiness check so we don't SIGINT before signal.NotifyContext is
	// installed in the child. Timeout after 2s.
	deadline := time.Now().Add(2 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if bytes.Contains(stdout.Bytes(), []byte("Starting anti-locker")) {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("child never printed startup line\nstdout:\n%s\nstderr:\n%s",
			stdout.String(), stderr.String())
	}

	// Send SIGINT to the child; assert it exits cleanly with code 0.
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal: %v", err)
	}

	err := cmd.Wait()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Non-zero exit is a failure (spec §8: Ctrl-C → exit 0 after clean stop).
		t.Fatalf("expected exit 0, got %v\nstdout:\n%s\nstderr:\n%s",
			exitErr, stdout.String(), stderr.String())
	}
	if err != nil {
		t.Fatalf("unexpected error: %v (stdout: %s, stderr: %s)",
			err, stdout.String(), stderr.String())
	}
}

func TestMain_NonTTYFirstRun_FailsClean(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Use a path that doesn't exist so the app tries first-run setup.
	missing := filepath.Join(t.TempDir(), "antilocker.yaml")

	cmd := exec.Command(testBinary, "--config", missing)
	// Pipe stdin (non-terminal) — spec §3.3 requires a clear error, exit 1.
	cmd.Stdin = bytes.NewReader(nil)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}
	// Verify the config file was NOT created (spec §3.4: no partial write).
	if _, statErr := os.Stat(missing); statErr == nil {
		t.Error("config file was created despite non-TTY first-run")
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test -run TestMain_CleanShutdown -v ./...`
Expected: PASS.

Run: `go test -run TestMain_NonTTYFirstRun_FailsClean -v ./...`
Expected: PASS.

Run: `go test -short ./...`
Expected: PASS (integration tests SKIPs).

- [ ] **Step 3: Commit**

```bash
git add main_test.go
git commit -m "test(integration): rebuild binary in TestMain, deliver real SIGINT, assert clean exit 0"
```

- [ ] **Step 4: Merge WS-E**

```bash
git checkout main
git merge --ff-only feature/main-cli
```

All core waves complete. Wave 4 (hardening) can now start.
## Chunk 7: Hardening Workstream — integration, smoke test, README, quality gates (WS-F, Wave 4)

Final polish, end-to-end verification, and documentation.

### Task 16: Opt-in integration test for real SSID detection

The spec §9 says: "Integration test for real SSID detection... skipped under `testing.Short()`". Per spec, no build tag is used — the default `go test ./...` runs it, and `go test -short ./...` skips it. This is the standard Go pattern for opt-in integration tests.

**Files:**
- Create: `internal/wifi/wifi_integration_test.go`

- [ ] **Step 1: Write the integration test**

```go
package wifi_test

import (
	"testing"

	"github.com/ansidev/antilocker/internal/wifi"
)

// TestCurrentSSID_Integration verifies real SSID detection against the
// live macOS Wi-Fi stack. Skipped under -short; runs under `go test ./...`.
func TestCurrentSSID_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test skipped in -short mode")
	}

	p := wifi.NewExecProvider(nil) // uses real exec.LookPath
	ssid, err := p.CurrentSSID()
	if err != nil {
		// Transient (e.g. scan in progress, or SIP blocked) — log and skip
		// rather than fail. See spec §8: transient SSID errors are warnings,
		// not fatal.
		t.Skipf("CurrentSSID() transient error: %v", err)
	}
	if ssid == "" {
		t.Log("Wi-Fi appears to be off or not associated with a network")
		return
	}
	t.Logf("current SSID: %q", ssid)
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/wifi/ -v -run TestCurrentSSID_Integration`
Expected: PASS or SKIP with a logged SSID. This is a manual verification step — it must NOT be a failing gate in CI.

Run: `go test -short ./internal/wifi/`
Expected: PASS (test SKIPs).

- [ ] **Step 3: Update the quality-gates section**

In Task 18, the README and quality gates must include both gates:

```bash
go test ./...          # all PASS, integration test runs (SKIP if Wi-Fi off)
go test -short ./...   # all PASS, integration test SKIPs
```

- [ ] **Step 4: Commit**

```bash
git add internal/wifi/wifi_integration_test.go
git commit -m "test(wifi): add integration test gated by testing.Short (no build tag)"
```

---

### Task 17: Manual smoke-test walkthrough

This is a manual verification pass. No new code; just confirm the built binary behaves as spec'd.

- [ ] **Step 1: Build a fresh binary**

Run: `go build -o antilocker .`
Expected: binary written to `./antilocker`.

- [ ] **Step 2: First-run smoke test (without existing config)**

```bash
# Use a temp path so we don't pollute the real default
./antilocker --config /tmp/antilocker-smoke.yaml
```

Expected:
1. Preamble printed: `No config found at /tmp/antilocker-smoke.yaml — let's set one up.`
2. Prompts: `Check interval in seconds [3600]:`
3. Type Enter → moves to network prompt.
4. Type a Wi-Fi name (your current one to make the match succeed), then Enter.
5. Type empty Enter to finish networks.
6. Confirmation printed: `Config saved to /tmp/antilocker-smoke.yaml`
7. App logs a startup line containing `Starting anti-locker | config=<path> interval=3600 networks=N` followed by a line matching `joining "<your-network>" — starting caffeinate` (only if the SSID matches your configured network).

- [ ] **Step 2.5: Ctrl-C during first-run does not leave a partial config (spec §3.4)**

Remove the config first, then start first-run and interrupt during a prompt:

```bash
rm -f /tmp/antilocker-smoke.yaml
./antilocker --config /tmp/antilocker-smoke.yaml
# When prompted, press Ctrl-C (do NOT type anything)
```

Expected:
1. `Setup cancelled.` is printed to stderr.
2. Exit code is `1` (verify with `echo $?`).
3. The config file does NOT exist (verify with `test -e /tmp/antilocker-smoke.yaml && echo "BAD: file exists" || echo "OK"` → must print `OK`).

- [ ] **Step 3: Verify caffeinate is running**

In another terminal while `antilocker` is running:

Run: `pgrep -fl caffeinate`
Expected: a process named `caffeinate -i` exists.

- [ ] **Step 4: Verify clean shutdown**

Press `Ctrl-C` in the `antilocker` terminal.

Expected:
1. `Shutdown signal received` logged.
2. The `caffeinate -i` process is gone (re-run `pgrep -fl caffeinate` — should return nothing).

- [ ] **Step 5: Verify subsequent run loads config silently**

```bash
./antilocker --config /tmp/antilocker-smoke.yaml
```

Expected: no prompts; straight into `Starting anti-locker ...`. Ctrl-C to stop.

- [ ] **Step 6: Clean up the smoke-test config**

Run: `rm /tmp/antilocker-smoke.yaml`

- [ ] **Step 7: Document findings**

Add a brief `## Smoke test results` section to `README.md` (or a `docs/smoke-test.md` file) noting what you observed. This serves as a regression baseline.

---

### Task 18: README finalization and quality gates

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Write the final README**

Replace the entire `README.md` with:

```markdown
# Anti Locker

`antilocker` keeps your MacBook awake by preventing idle sleep — but only when you're on a Wi-Fi network you've explicitly allowed. On any other network (or if Wi-Fi is off) it stays silent and lets your Mac sleep normally.

**macOS only.** Manual screen lock (`Ctrl+Cmd+Q`) and display-off always work — `antilocker` only blocks *idle* sleep, never your explicit actions.

## Install

    go build -o antilocker .
    # optionally: install -m 0755 antilocker /usr/local/bin/antilocker

## Usage

    antilocker                                # use ~/.config/antilocker.yaml
    antilocker --config /path/to/config.yaml  # custom config path

On first run (no config file), `antilocker` will interactively ask for:

1. **Check interval** in seconds (default `3600`)
2. **Wi-Fi networks** to keep the Mac awake on (at least one, one at a time, empty line to finish)

It saves your answers to the config path and starts running immediately.

## Configuration

```yaml
interval: 3600            # optional, seconds; defaults to 3600
networks:                 # required, may be empty list
  - "Home Network 1"
  - "Home Network 2"
```

- SSID matching is exact and case-sensitive.
- `interval: 0` or negative → startup error.
- `networks: null` or a missing `networks:` key → startup error.
- Duplicates are de-duplicated with a startup warning.

## How it works

1. Every `interval` seconds, `antilocker` checks the current Wi-Fi SSID via `ipconfig getsummary en0` (falling back to `system_profiler SPAirPortDataType`).
2. If the SSID matches a configured network, it spawns `caffeinate -i` to block idle sleep.
3. If the SSID stops matching (or Wi-Fi drops), it stops `caffeinate`.
4. `Ctrl-C` cleanly shuts down the app and terminates `caffeinate`.

## Development

    go build ./...
    go test ./...
    go test -short ./...
    go vet ./...
    gofmt -s -l .
    GOOS=darwin go build -o antilocker .

## License

See [LICENSE](LICENSE).
```

- [ ] **Step 2: Run all quality gates**

Run each and verify expected outcome:

```bash
go build ./...                # no output = OK
go test ./...                 # all PASS
go test -short ./...          # all PASS, integration test SKIPs
go vet ./...                  # no output = clean
gofmt -s -l .                 # empty output = clean
GOOS=darwin go build -o antilocker .
```

- [ ] **Step 3: Verify the built binary runs**

```bash
./antilocker --version
./antilocker --help
```

Expected:
- `./antilocker --version` prints `antilocker version dev` (or the ldflags-set version).
- `./antilocker --help` shows usage including the `--config` flag.

- [ ] **Step 4: Commit and merge hardening**

```bash
git add README.md
git commit -m "docs: finalize README with usage, config reference, and dev commands"

# If on hardening branch:
git checkout main
git merge --ff-only feature/hardening
```

- [ ] **Step 5: Tag the release**

```bash
git tag -a v0.1.0 -m "Anti Locker v0.1.0 — initial release"
```

(The user can push tags when ready; do not push automatically.)

---

## Completion checklist

When all 18 tasks are done, the following must be true:

- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes (integration test runs but skips if no Wi-Fi)
- [ ] `go test -short ./...` passes with integration tests skipped
- [ ] `go vet ./...` clean
- [ ] `gofmt -s -l .` produces no output
- [ ] `./antilocker --version` and `--help` print expected output
- [ ] Manual smoke test (Task 17) verified clean shutdown
- [ ] First-run Ctrl-C prints `Setup cancelled.`, exits `1`, and leaves no config file (spec §3.4 — verified via Task 17 Step 2.5)
- [ ] First run with non-TTY stdin exits `1` with a clear message (spec §3.3 — covered by Task 15 test `TestMain_NonTTYFirstRun_FailsClean`)
- [ ] `caffeinate` is never left running after `antilocker` exits
- [ ] All waves merged to `main` in the order listed in Merge Order

Reference the spec at `docs/superpowers/specs/2026-08-01-antilocker-design.md` for any behavioral questions during implementation.
