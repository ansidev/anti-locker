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
