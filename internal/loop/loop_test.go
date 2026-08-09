package loop_test

import (
	"context"
	"testing"
	"time"

	"github.com/ansidev/anti-locker/internal/config"
	"github.com/ansidev/anti-locker/internal/loop"
)

type fakeWiFi struct {
	ssids []string
	idx   int
	err   error
}

func (f *fakeWiFi) CurrentSSID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	ssid := f.ssids[f.idx]
	if f.idx+1 < len(f.ssids) {
		f.idx++
	}
	return ssid, nil
}

type fakeKeepAwake struct {
	started []string
	stopped int
	running bool
}

func (f *fakeKeepAwake) Start(network string) error {
	f.started = append(f.started, network)
	f.running = true
	return nil
}

func (f *fakeKeepAwake) Stop() {
	if f.running {
		f.stopped++
		f.running = false
	}
}

func (f *fakeKeepAwake) IsRunning() bool { return f.running }

func TestRun_MatchStartsKeepAwake(t *testing.T) {
	w := &fakeWiFi{ssids: []string{"Home Wi-Fi", "Home Wi-Fi", "Home Wi-Fi"}}
	k := &fakeKeepAwake{}
	cfg := &config.Config{Interval: 1, Networks: []string{"Home Wi-Fi"}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(3500 * time.Millisecond) // allow 3 ticks at 1s interval
		cancel()
	}()
	loop.Run(ctx, cfg, w, k)

	if len(k.started) != 1 {
		t.Errorf("len(started) = %d, want 1", len(k.started))
	}
	// The keep-awake must stay running across all matching ticks and only
	// be stopped by the guaranteed shutdown stop. The plan's original
	// assertion of 0 conflicts with Run's ctx.Done() handler (which always
	// calls Stop for cleanup); 1 = the shutdown stop only.
	if k.stopped != 1 {
		t.Errorf("stopped count = %d, want 1 (shutdown stop only)", k.stopped)
	}
}

func TestRun_UnmatchStopsKeepAwake(t *testing.T) {
	w := &fakeWiFi{ssids: []string{"Home Wi-Fi", "Office Wi-Fi"}}
	k := &fakeKeepAwake{}
	cfg := &config.Config{Interval: 1, Networks: []string{"Home Wi-Fi"}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(2500 * time.Millisecond) // allow 2 ticks: match then unmatch
		cancel()
	}()
	loop.Run(ctx, cfg, w, k)

	if len(k.started) != 1 {
		t.Errorf("len(started) = %d, want 1", len(k.started))
	}
	if k.stopped != 1 {
		t.Errorf("stopped count = %d, want 1", k.stopped)
	}
}

func TestRun_SignalStopsKeepAwake(t *testing.T) {
	w := &fakeWiFi{ssids: []string{"Home Wi-Fi"}}
	k := &fakeKeepAwake{}
	cfg := &config.Config{Interval: 10, Networks: []string{"Home Wi-Fi"}}

	ctx, cancel := context.WithCancel(context.Background())

	// Start loop, trigger one check, then cancel to simulate SIGINT/SIGTERM
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	loop.Run(ctx, cfg, w, k)

	if len(k.started) != 1 {
		t.Errorf("len(started) = %d, want 1", len(k.started))
	}
	if k.stopped != 1 {
		t.Errorf("stopped count = %d, want 1 (should be stopped on shutdown)", k.stopped)
	}
}
