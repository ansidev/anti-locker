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
