package wifi_test

import (
	"fmt"
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

func TestCurrentSSID_SystemProfilerFallback(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			if name == "ipconfig" {
				return "", fmt.Errorf("ipconfig missing")
			}
			return "Current Network Information:\n    Network Name : Home Wi-Fi\n", nil
		},
	)
	ssid, err := p.CurrentSSID()
	if err != nil {
		t.Fatalf("CurrentSSID() unexpected error: %v", err)
	}
	if ssid != "Home Wi-Fi" {
		t.Errorf("ssid = %q, want %q", ssid, "Home Wi-Fi")
	}
}

func TestCurrentSSID_WiFiOff(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			if name == "ipconfig" {
				return "", fmt.Errorf("ipconfig missing")
			}
			return "Current Network Information:\n    Network Name :\n", nil
		},
	)
	ssid, err := p.CurrentSSID()
	if err != nil {
		t.Fatalf("CurrentSSID() unexpected error: %v", err)
	}
	if ssid != "" {
		t.Errorf("ssid = %q, want empty string", ssid)
	}
}

func TestCurrentSSID_MissingBinaries(t *testing.T) {
	p := wifi.NewExecProvider(
		func(name string) (string, error) {
			return "", fmt.Errorf("%s not found", name)
		},
	)
	_, err := p.CurrentSSID()
	if err == nil {
		t.Fatal("expected error when both binaries missing")
	}
}
