package wifi

import (
	"context"

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
	runCmd func(string) (string, error)
}

// NewExecProvider returns an ExecProvider that uses runCmd to
// execute macOS binaries and return their output. Pass nil to use
// the real exec-based runner.
func NewExecProvider(runCmd func(string) (string, error)) *ExecProvider {
	return &ExecProvider{runCmd: runCmd}
}

func (p *ExecProvider) CurrentSSID() (string, error) {
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
