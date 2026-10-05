package wifi

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jaisonerick/macwifi"
)

// Provider is the seam injected into internal/loop for testing.
type Provider interface {
	CurrentSSID() (string, error)
}

// ExecProvider is the production implementation; it shells out to
// macOS binaries. runCmd is injectable so unit tests never touch
// real exec. Production wiring passes nil to use a real exec runner.
type ExecProvider struct {
	runCmd func() (string, error)
}

// NewExecProvider returns an ExecProvider that uses runCmd to
// execute macOS binaries and return their output. Pass nil to use
// the real exec-based runner.
func NewExecProvider(runCmd func() (string, error)) *ExecProvider {
	if runCmd == nil {
		runCmd = runRealWithRecovery
	}
	return &ExecProvider{runCmd: runCmd}
}

func runReal() (string, error) {
	networks, err := macwifi.Scan(context.Background())
	if err != nil {
		return "", err
	}

	for _, network := range networks {
		if network.Current {
			return network.SSID, nil
		}
	}

	return "", err // transient warning by loop
}

// CurrentSSID returns the current network's SSID, or an error.
func (p *ExecProvider) CurrentSSID() (string, error) {
	return p.runCmd()
}

// runRealWithRecovery looks the SSID up via the macwifi helper and
// recovers from a known macOS failure mode: launchd can pin a stuck
// launch request to the extracted WifiScanner.app bundle, so every
// subsequent launch fails and macwifi only reports an accept timeout.
// Deleting the extraction cache forces a fresh extraction, then the
// lookup is retried once. If the helper still fails, the spec'd
// ipconfig fallback (spec §5) runs before giving up.
func runRealWithRecovery() (string, error) {
	heal := func() error {
		if err := invalidateMacwifiCache(); err != nil {
			return err
		}
		log.Printf("wifi: helper launch failed; cleared extraction cache to force a fresh helper")
		return nil
	}
	return lookupWithRecovery(runReal, heal, ssidViaIPConfig)
}

// lookupWithRecovery is the recovery strategy; its steps are injectable
// for unit tests. The original scan error is returned when every
// recovery path fails, so the loop logs the primary cause.
func lookupWithRecovery(scan func() (string, error), heal func() error, fallback func() (string, error)) (string, error) {
	ssid, err := scan()
	if err == nil {
		return ssid, nil
	}

	if heal != nil && heal() == nil {
		if ssid, retryErr := scan(); retryErr == nil {
			return ssid, nil
		}
	}

	if fallback != nil {
		if ssid, fbErr := fallback(); fbErr == nil {
			return ssid, nil
		}
	}

	return "", err
}

// invalidateMacwifiCache removes the extracted WifiScanner.app bundle
// so the next macwifi call re-extracts it. The library recreates the
// cache deterministically from its embedded, signed bundle, so removal
// is safe at any time.
func invalidateMacwifiCache() error {
	base, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(base, "macwifi")
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(dir)
}

// ssidViaIPConfig reads the current SSID from `ipconfig getsummary` for
// the Wi-Fi interface. Modern macOS redacts the value unless the calling
// process holds Location Services permission; redacted or missing values
// surface as an error and the caller reports the primary helper error.
func ssidViaIPConfig() (string, error) {
	iface, err := wifiInterface()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("ipconfig", "getsummary", iface).Output()
	if err != nil {
		return "", err
	}
	return parseIPConfigSSID(string(out))
}

// parseIPConfigSSID extracts the "SSID : value" line from
// `ipconfig getsummary` output. BSSID lines are ignored by anchoring on
// the line start; redacted or empty values count as not-found.
func parseIPConfigSSID(out string) (string, error) {
	const key = "SSID : "
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, key) {
			continue
		}
		ssid := strings.TrimSpace(trimmed[len(key):])
		if ssid != "" && ssid != "<redacted>" {
			return ssid, nil
		}
	}
	return "", errors.New("wifi: no usable SSID in ipconfig output")
}

// wifiInterface resolves the Wi-Fi device from networksetup, falling
// back to en0 (the conventional macOS Wi-Fi device) when networksetup
// itself fails.
func wifiInterface() (string, error) {
	out, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return "en0", nil
	}
	if iface := parseWifiDevice(string(out)); iface != "" {
		return iface, nil
	}
	return "", errors.New("wifi: no Wi-Fi interface found in networksetup output")
}

// parseWifiDevice returns the Device of the "Wi-Fi" Hardware Port.
func parseWifiDevice(out string) string {
	var inWifiPort bool
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Hardware Port:"):
			port := strings.TrimSpace(strings.TrimPrefix(trimmed, "Hardware Port:"))
			inWifiPort = port == "Wi-Fi"
		case inWifiPort && strings.HasPrefix(trimmed, "Device:"):
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Device:"))
		}
	}
	return ""
}
