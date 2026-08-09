package main_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// testBinary is built once by TestMain (skipped in -short mode).
var testBinary string

func TestMain(m *testing.M) {
	// testing.Short() panics unless flags are parsed first.
	flag.Parse()
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
	defer func() { _ = os.RemoveAll(dir) }()

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

// safeBuffer is a mutex-protected bytes.Buffer for concurrent use
// (exec.Cmd's pipe goroutines write while the test goroutine reads).
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) Contains(b []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Contains(s.buf.Bytes(), b)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
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

	stdout := &safeBuffer{}
	stderr := &safeBuffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for the child to print its startup line. This is a lightweight
	// readiness check so we don't SIGINT before signal.NotifyContext is
	// installed in the child. Timeout after 2s. log.Printf writes to stderr,
	// so poll stderr (the plan's stdout check was a bug).
	deadline := time.Now().Add(2 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if stderr.Contains([]byte("Starting anti-locker")) {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		// Kill and reap the child first to stop exec's pipe writers,
		// then read the buffers safely.
		_ = cmd.Process.Kill()
		if err := cmd.Wait(); err != nil {
			t.Logf("cmd.Wait after kill: %v", err)
		}
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
