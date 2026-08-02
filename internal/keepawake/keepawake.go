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
	mu       sync.Mutex
	cmd      *exec.Cmd
	alive    bool          // true while child is running; cleared by reaper goroutine
	reapDone chan struct{} // closed when reaper's Wait() returns; Stop waits on this
	newCmd   CommandFactory
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
	// The reaper clears `alive` only after Wait() finishes, so if we see
	// !alive and a non-nil cmd, the child has already exited and Wait() is
	// done — no race with ProcessState.
	if m.cmd != nil && !m.alive {
		m.cmd = nil
	}
	if m.cmd != nil {
		// Defensive: alive==false but cmd non-nil — treat as starting; refuse.
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

	// Reaper goroutine: the ONLY caller of cmd.Wait(). This avoids the
	// race between a second Wait in Stop's grace goroutine and here.
	// It signals completion by closing reapDone so Stop can wait on it.
	reapDone := make(chan struct{})
	m.reapDone = reapDone
	go func(c *exec.Cmd) {
		_ = c.Wait()
		m.mu.Lock()
		if m.cmd == c {
			m.alive = false
		}
		m.mu.Unlock()
		close(reapDone)
	}(cmd)

	return nil
}

// Stop terminates the caffeinate process. It is a no-op if already stopped.
//
// Sequence (spec §6.4): SIGTERM → short grace period → SIGKILL.
// The reaper goroutine (from Start) is the sole caller of cmd.Wait(), so
// Stop waits on reapDone for the child to be reaped instead of calling
// Wait again (which would race).
func (m *ExecManager) Stop() {
	m.mu.Lock()
	cmd := m.cmd
	reapDone := m.reapDone
	// Mark not-alive immediately so concurrent IsRunning reports false
	// while we SIGTERM/KILL.
	m.alive = false
	m.cmd = nil
	m.reapDone = nil
	m.mu.Unlock()

	if cmd == nil {
		return // already stopped
	}
	// No ProcessState check here: reading it concurrently with the reaper's
	// Wait() would itself be a data race. If the child already exited, the
	// reaper has closed reapDone, so the select below returns immediately.

	// 1. Send SIGTERM (spec §6.4).
	_ = cmd.Process.Signal(syscall.SIGTERM)

	// 2. Grace period for clean exit. The reaper's Wait() is what reaps;
	//    we just wait for it to signal completion.
	grace := 100 * time.Millisecond
	select {
	case <-reapDone:
		return
	case <-time.After(grace):
		_ = cmd.Process.Kill()
		<-reapDone // reap the child to avoid zombies
	}
}

// IsRunning reports whether caffeinate is currently active.
// Reflects real process liveness via the reaper goroutine.
func (m *ExecManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alive
}
