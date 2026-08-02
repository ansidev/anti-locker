package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ansidev/antilocker/internal/config"
)

func TestLoad_HappyPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 600
networks:
  - "My Home"
  - "Office Wi-Fi"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if got.Interval != 600 {
		t.Errorf("Interval = %d, want 600", got.Interval)
	}
	if len(got.Networks) != 2 {
		t.Errorf("len(Networks) = %d, want 2", len(got.Networks))
	}
	if got.Networks[0] != "My Home" {
		t.Errorf("Networks[0] = %q, want %q", got.Networks[0], "My Home")
	}
}

func TestLoad_MissingNetworksKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 600
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing networks key")
	}
}

func TestLoad_IntervalZero(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 0
networks:
  - "Home"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for interval <= 0")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: high
networks:
  - "Home"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for malformed YAML type")
	}
}

func TestLoad_NullNetworks(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks: null
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for null networks")
	}
}

// Spec §4.3: "SSID whitespace: Trimmed at load time."
func TestLoad_TrimsSSIDWhitespace(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks:
  - "  Padded Network  "
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Networks[0] != "Padded Network" {
		t.Errorf("Networks[0] = %q, want trimmed %q", cfg.Networks[0], "Padded Network")
	}
}

func TestLoad_EmptyNetworkEntry(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks:
  - ""
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for empty network entry")
	}
}

func TestContains_Match(t *testing.T) {
	cfg := &config.Config{Networks: []string{"Home Network 1", "Home Network 2"}}
	if !cfg.Contains("Home Network 1") {
		t.Error("expected Contains to return true for matching SSID")
	}
}

func TestContains_NoMatch(t *testing.T) {
	cfg := &config.Config{Networks: []string{"Home Network 1"}}
	if cfg.Contains("Office Wi-Fi") {
		t.Error("expected Contains to return false for non-matching SSID")
	}
}

func TestContains_EmptyNetworks(t *testing.T) {
	cfg := &config.Config{Networks: []string{}}
	if cfg.Contains("Home Network 1") {
		t.Error("expected Contains to return false for empty networks list")
	}
}

// Spec §4.2: "interval: 3600  # optional, seconds; defaults to 3600"
func TestLoad_DefaultInterval(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`networks:
  - "Home"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Interval != 3600 {
		t.Errorf("Interval = %d, want 3600 (default)", cfg.Interval)
	}
}

// Spec §4.3: explicit networks: [] is valid and matches nothing.
func TestLoad_EmptyNetworksListIsValid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "antilocker.yaml")
	if err := os.WriteFile(cfgPath, []byte(`interval: 3600
networks: []
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Networks) != 0 {
		t.Errorf("len(Networks) = %d, want 0", len(cfg.Networks))
	}
	if cfg.Contains("anything") {
		t.Error("Contains returned true for empty list")
	}
}
