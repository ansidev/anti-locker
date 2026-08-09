package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ansidev/anti-locker/internal/config"
	"github.com/ansidev/anti-locker/internal/keepawake"
	"github.com/ansidev/anti-locker/internal/loop"
	"github.com/ansidev/anti-locker/internal/wifi"
	"github.com/urfave/cli/v3"
)

var version = "dev" // overridden by ldflags

func main() {
	app := &cli.Command{
		Name:    "anti-locker",
		Usage:   "keep macOS awake conditionally based on Wi-Fi network",
		Version: version,
		// Description surfaces an example config in --help output (spec §6.1).
		Description: `Keep this Mac awake when connected to one of your trusted Wi-Fi networks.

Example config (~/.config/anti-locker.yaml):

  interval: 3600
  networks:
    - "Home Network 1"
    - "Home Network 2"
`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Usage: "path to YAML config file",
				Value: "~/.config/anti-locker.yaml",
			},
		},
		Action: runAction,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runAction is the urfave/cli action handler. Signal wiring is set up
// first so that Ctrl-C / SIGTERM during first-run setup is caught.
func runAction(ctx context.Context, c *cli.Command) error {
	cfgPath := expandPath(c.String("config"))

	// Wire SIGINT/SIGTERM to context cancellation before any I/O so that
	// first-run setup can be interrupted cleanly (spec §3.4).
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			// First run. Require an interactive TTY (spec §3.3).
			if !stdinIsTerminal() {
				return fmt.Errorf("no config found at %s and stdin is not a terminal; please run interactively to complete first-run setup", cfgPath)
			}
			cfg, err = config.Run(ctx, cfgPath, os.Stdin, os.Stdout)
			if err != nil {
				if errors.Is(err, config.ErrCancelled) {
					// Spec §3.4: user cancelled during first-run setup.
					fmt.Fprintln(os.Stderr, "Setup cancelled.")
					return err
				}
				return fmt.Errorf("first-run setup failed: %w", err)
			}
		case errors.Is(err, config.ErrValidation):
			return fmt.Errorf("config validation failed: %w", err)
		default:
			return fmt.Errorf("failed to load config %s: %w", cfgPath, err)
		}
	}

	if err := keepawake.VerifyCaffeinate(); err != nil {
		return fmt.Errorf("fatal: %w", err)
	}

	mgr := keepawake.NewExecManager()
	defer mgr.Stop() // guarantee caffeinate never outlives anti-locker (spec §7)

	w := wifi.NewExecProvider(nil)

	loop.Run(ctx, cfg, w, mgr)
	return nil
}

// expandPath resolves "~/..." to the user's home directory.
// If home lookup fails, the raw path is returned unchanged.
func expandPath(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// stdinIsTerminal reports whether stdin is a terminal (spec §3.3).
// Uses pure stdlib; no extra dependency.
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
