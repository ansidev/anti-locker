package wifi

import (
	"errors"
	"testing"
)

func TestLookupWithRecovery(t *testing.T) {
	primaryErr := errors.New("macwifi: waiting for helper to connect")

	tests := map[string]struct {
		scanErrs      []error // consumed in order; exhausted → nil error
		healErr       error
		fallbackSSID  string
		fallbackErr   error
		wantSSID      string
		wantPrimaryEr bool // want the original scan error back
	}{
		"scan succeeds immediately": {
			scanErrs: []error{},
			wantSSID: "Home",
		},
		"heal clears stuck helper": {
			scanErrs: []error{primaryErr},
			wantSSID: "Home",
		},
		"heal fails, fallback succeeds": {
			scanErrs:     []error{primaryErr},
			healErr:      errors.New("remove failed"),
			fallbackSSID: "Home",
			wantSSID:     "Home",
		},
		"heal ok but retry fails, fallback succeeds": {
			scanErrs:     []error{primaryErr, primaryErr},
			fallbackSSID: "Home",
			wantSSID:     "Home",
		},
		"all paths fail returns primary error": {
			scanErrs:      []error{primaryErr, primaryErr},
			fallbackErr:   errors.New("redacted"),
			wantPrimaryEr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scanErrs := tc.scanErrs
			scan := func() (string, error) {
				if len(scanErrs) > 0 {
					err := scanErrs[0]
					scanErrs = scanErrs[1:]
					return "", err
				}
				return "Home", nil
			}
			heal := func() error { return tc.healErr }
			fallback := func() (string, error) { return tc.fallbackSSID, tc.fallbackErr }

			ssid, err := lookupWithRecovery(scan, heal, fallback)

			if tc.wantPrimaryEr {
				if !errors.Is(err, primaryErr) {
					t.Fatalf("err = %v, want primary scan error %v", err, primaryErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ssid != tc.wantSSID {
				t.Errorf("ssid = %q, want %q", ssid, tc.wantSSID)
			}
		})
	}
}

func TestParseIPConfigSSID(t *testing.T) {
	tests := map[string]struct {
		out     string
		want    string
		wantErr bool
	}{
		"extracts SSID with spaces": {
			out:  "  BSSID : aa:bb:cc:dd:ee:ff\n  SSID : My Home Wi-Fi 5G\n  Security : WPA2_PSK\n",
			want: "My Home Wi-Fi 5G",
		},
		"ignores BSSID lines": {
			out:     "  BSSID : aa:bb:cc:dd:ee:ff\n",
			wantErr: true,
		},
		"redacted is not found": {
			out:     "  SSID : <redacted>\n",
			wantErr: true,
		},
		"missing is not found": {
			out:     "  ConnectionID : 4\n",
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ssid, err := parseIPConfigSSID(tc.out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want error (ssid = %q)", ssid)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ssid != tc.want {
				t.Errorf("ssid = %q, want %q", ssid, tc.want)
			}
		})
	}
}

func TestParseWifiDevice(t *testing.T) {
	out := `Hardware Port: Wi-Fi
Device: en0
Ethernet Address: aa:bb:cc:dd:ee:ff

Hardware Port: Ethernet
Device: en5
Ethernet Address: 11:22:33:44:55:66
`
	if got := parseWifiDevice(out); got != "en0" {
		t.Errorf("parseWifiDevice() = %q, want %q", got, "en0")
	}
	if got := parseWifiDevice("Hardware Port: Ethernet\nDevice: en5\n"); got != "" {
		t.Errorf("parseWifiDevice() = %q, want empty", got)
	}
}
