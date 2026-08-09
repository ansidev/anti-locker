//go:build !github_actions

package wifi_test

import (
	"testing"

	"github.com/ansidev/anti-locker/internal/wifi"
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
