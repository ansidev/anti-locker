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
