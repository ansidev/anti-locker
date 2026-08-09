package config_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ansidev/antilocker/internal/config"
)

func TestRun_SetupPrompts(t *testing.T) {
	// Empty interval input → default 3600; one network; then empty to finish.
	inputStr := "\nHome Wi-Fi\n\n"
	var buf bytes.Buffer

	cfg, err := config.Run(context.Background(), filepath.Join(t.TempDir(), "antilocker.yaml"), strings.NewReader(inputStr), &buf)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if cfg.Interval != 3600 {
		t.Errorf("Interval = %d, want 3600 (default)", cfg.Interval)
	}
	if len(cfg.Networks) != 1 || cfg.Networks[0] != "Home Wi-Fi" {
		t.Errorf("Networks = %v, want [Home Wi-Fi]", cfg.Networks)
	}
	// Spec §3.3 step 1: preamble line must be printed.
	if !strings.Contains(buf.String(), "No config found at") {
		t.Errorf("expected preamble in output, got %q", buf.String())
	}
	// Spec §3.3 step 4: saved-path line must be printed.
	if !strings.Contains(buf.String(), "Config saved to") {
		t.Errorf("expected saved-path message in output, got %q", buf.String())
	}
}

func TestRun_InvalidIntervalRePrompts(t *testing.T) {
	// Three invalid interval attempts then a valid one; then a network, then empty to finish.
	inputStr := "abc\n-1\n0\n600\nHome Wi-Fi\n\n"
	var buf bytes.Buffer

	cfg, err := config.Run(context.Background(), filepath.Join(t.TempDir(), "antilocker.yaml"), strings.NewReader(inputStr), &buf)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if cfg.Interval != 600 {
		t.Errorf("Interval = %d, want 600", cfg.Interval)
	}
}

func TestRun_NetworkRequired(t *testing.T) {
	// First empty input is rejected (re-prompt), then a real one is accepted.
	inputStr := "600\n\nHome Wi-Fi\n\n"
	var buf bytes.Buffer

	cfg, err := config.Run(context.Background(), filepath.Join(t.TempDir(), "antilocker.yaml"), strings.NewReader(inputStr), &buf)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(cfg.Networks) != 1 || cfg.Networks[0] != "Home Wi-Fi" {
		t.Errorf("Networks = %v, want [Home Wi-Fi]", cfg.Networks)
	}
}

func TestRun_CancelledByContext(t *testing.T) {
	// Spec §3.4: cancelled context must return ErrCancelled without creating a file.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before Run starts

	target := filepath.Join(t.TempDir(), "antilocker.yaml")
	var buf bytes.Buffer

	_, err := config.Run(ctx, target, strings.NewReader(""), &buf)
	if err == nil {
		t.Fatal("expected ErrCancelled")
	}
	if !errors.Is(err, config.ErrCancelled) {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Error("no partial config should be written when cancelled")
	}
}
