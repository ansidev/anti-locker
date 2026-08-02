package loop

import (
	"context"
	"log"
	"time"

	"github.com/ansidev/antilocker/internal/config"
	"github.com/ansidev/antilocker/internal/keepawake"
	"github.com/ansidev/antilocker/internal/wifi"
)

// Run executes the periodic check loop until ctx is cancelled.
// It has no return value: context cancellation / SIGINT/SIGTERM is normal
// shutdown (exit 0 handled by caller), and all recoverable runtime issues
// are logged as warnings. Fatal configuration problems are caught before
// Run is invoked.
func Run(ctx context.Context, cfg *config.Config, w wifi.Provider, k keepawake.Manager) {
	// Spec §6.5 step 1: log startup summary including config path
	log.Printf("Starting anti-locker | config=%s interval=%ds networks=%d", cfg.Path, cfg.Interval, len(cfg.Networks))

	// Immediate check before first tick
	check(cfg, w, k)

	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown signal received")
			k.Stop()
			return
		case <-ticker.C:
			check(cfg, w, k)
		}
	}
}

// check performs one SSID lookup and updates keep-awake state.
func check(cfg *config.Config, w wifi.Provider, k keepawake.Manager) {
	ssid, err := w.CurrentSSID()
	if err != nil {
		log.Printf("warn: ssid lookup failed: %v", err)
		return
	}

	matched := cfg.Contains(ssid)
	running := k.IsRunning()

	switch {
	case matched && !running:
		log.Printf("joining %q — starting caffeinate", ssid)
		if err := k.Start(ssid); err != nil {
			log.Printf("warn: caffeinate start failed: %v", err)
		}
	case !matched && running:
		log.Printf("left network — stopping caffeinate")
		k.Stop()
	}
}
