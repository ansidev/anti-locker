package keepawake_test

// TestMain's helper process pattern: when invoked with -test.run=TestHelperProcess
// and GO_HELPER_PROCESS=1, the test binary itself acts as the fake command.
// This lets us fake exec.Cmd without spawning real caffeinate.

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ansidev/anti-locker/internal/keepawake"
)

func TestVerifyCaffeinate_Found(t *testing.T) {
	// On macOS, caffeinate is guaranteed to exist. In unit tests we rely
	// on the real PATH lookup. If this fails on your machine, skip it
	// or set PATH to include /usr/bin.
	if err := keepawake.VerifyCaffeinate(); err != nil {
		t.Fatalf("VerifyCaffeinate() unexpected error: %v", err)
	}
}

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
