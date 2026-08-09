package config

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrCancelled is returned when the user aborts first-run setup
// (EOF on stdin, or upstream signal handling). Callers should print
// "Setup cancelled." (spec §3.4) and exit with code 1.
var ErrCancelled = errors.New("setup cancelled")

// Run executes the interactive first-run setup. It reads prompts from
// `reader` (os.Stdin in production) and writes prompts to `w`.
//
// It accepts ctx so SIGINT/SIGTERM cancellation is respected (spec §3.4):
// internally it runs a goroutine that pushes scanned lines into a channel,
// letting it select between input and ctx.Done(). When cancelled, it
// returns ErrCancelled without writing any file (spec §3.4).
//
// It prints a preamble line ("No config found at <path> — let's set one up.")
// before prompting (spec §3.3 step 1), persists a validated YAML config to
// path, prints "Config saved to <path>" on success (spec §3.3 step 4), and
// returns the Config.
func Run(ctx context.Context, path string, reader io.Reader, w io.Writer) (*Config, error) {
	if _, err := fmt.Fprintf(w, "No config found at %s — let's set one up.\n", path); err != nil {
		return nil, fmt.Errorf("writing preamble: %w", err)
	}

	lines := make(chan string, 16)
	scanErrs := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		// EOF or read error ends the stream.
		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			scanErrs <- err
		}
		close(lines)
	}()

	readLine := func() (string, error) {
		select {
		case <-ctx.Done():
			return "", ErrCancelled
		case err := <-scanErrs:
			return "", err
		case line, ok := <-lines:
			if !ok {
				return "", ErrCancelled // stdin closed (Ctrl-D / EOF)
			}
			return line, nil
		}
	}

	interval, err := promptInterval(readLine, w)
	if err != nil {
		return nil, err
	}

	networks, err := promptNetworks(readLine, w)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Path:     path,
		Interval: interval,
		Networks: networks,
	}

	if err := persistConfig(path, cfg); err != nil {
		return nil, err
	}

	if _, err := fmt.Fprintf(w, "Config saved to %s\n", path); err != nil {
		return nil, fmt.Errorf("writing confirmation: %w", err)
	}
	return cfg, nil
}

// promptInterval reads and validates the check interval. Re-prompts on
// invalid input. Returns ErrCancelled when readLine fails.
func promptInterval(readLine func() (string, error), w io.Writer) (int, error) {
	for {
		if _, err := fmt.Fprint(w, "Check interval in seconds [3600]: "); err != nil {
			return 0, err
		}
		raw, err := readLine()
		if err != nil {
			return 0, err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return 3600, nil
		}
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
			if _, err := fmt.Fprintln(w, "Please enter a positive integer."); err != nil {
				return 0, err
			}
			continue
		}
		return n, nil
	}
}

// promptNetworks collects Wi-Fi names one at a time. Returns ErrCancelled
// when readLine fails. Requires at least one network (spec §3.3 step 3).
func promptNetworks(readLine func() (string, error), w io.Writer) ([]string, error) {
	var networks []string
	for {
		if _, err := fmt.Fprint(w, "Add a network name (leave empty to finish): "); err != nil {
			return nil, err
		}
		raw, err := readLine()
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(raw)
		if name == "" {
			if len(networks) == 0 {
				if _, err := fmt.Fprintln(w, "At least one network is required."); err != nil {
					return nil, err
				}
				continue
			}
			return networks, nil
		}
		networks = append(networks, name)
	}
}

func persistConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
