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
// macOS binaries. runCmd is injectable so unit tests never touch
// real exec. Production wiring passes nil to use a real exec runner.
type ExecProvider struct {
	runCmd func(string) (string, error)
}

// NewExecProvider returns an ExecProvider that uses runCmd to
// execute macOS binaries and return their output. Pass nil to use
// the real exec-based runner.
func NewExecProvider(runCmd func(string) (string, error)) *ExecProvider {
	if runCmd == nil {
		runCmd = runReal
	}
	return &ExecProvider{runCmd: runCmd}
}

// runReal executes name with the arguments CurrentSSID expects and
// returns stdout as a string.
func runReal(name string) (string, error) {
	var args []string
	switch name {
	case "ipconfig":
		args = []string{"getsummary", "en0"}
	case "system_profiler":
		args = []string{"SPAirPortDataType"}
	default:
		return "", fmt.Errorf("unsupported binary: %s", name)
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found: %w", name, err)
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (p *ExecProvider) CurrentSSID() (string, error) {
	ipOut, err := p.runCmd("ipconfig")
	if err != nil {
		ipOut = "" // fall through to system_profiler
	}

	if ssid := parseIPConfig([]byte(ipOut)); ssid != "" {
		return ssid, nil
	}

	spOut, err := p.runCmd("system_profiler")
	if err != nil {
		return "", err // transient warning by loop
	}
	return parseSystemProfiler([]byte(spOut)), nil
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
