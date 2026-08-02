package keepawake

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
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
