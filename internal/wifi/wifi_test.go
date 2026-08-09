package wifi_test

import (
	"fmt"
	"testing"

	"github.com/ansidev/anti-locker/internal/wifi"
)

func TestCurrentSSID_MacWifi(t *testing.T) {
	p := wifi.NewExecProvider(
		func() (string, error) {
			return "My Home Wi-Fi", nil
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

func TestCurrentSSID_WiFiOff(t *testing.T) {
	p := wifi.NewExecProvider(
		func() (string, error) {
			return "", fmt.Errorf("error")
		},
	)
	ssid, err := p.CurrentSSID()
	if err == nil {
		t.Fatalf("error = %v, want error", err)
	}
	if ssid != "" {
		t.Errorf("ssid = %q, want empty string", ssid)
	}
}
