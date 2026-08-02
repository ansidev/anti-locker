package keepawake_test

import (
	"testing"

	"github.com/ansidev/antilocker/internal/keepawake"
)

func TestVerifyCaffeinate_Found(t *testing.T) {
	// On macOS, caffeinate is guaranteed to exist. In unit tests we rely
	// on the real PATH lookup. If this fails on your machine, skip it
	// or set PATH to include /usr/bin.
	if err := keepawake.VerifyCaffeinate(); err != nil {
		t.Fatalf("VerifyCaffeinate() unexpected error: %v", err)
	}
}
