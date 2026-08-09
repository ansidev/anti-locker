package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrValidation marks configuration-validation failures so callers can
// distinguish them from IO or parse errors.
var ErrValidation = errors.New("config validation error")

// Config holds the parsed anti-locker.yaml contents.
// Path records the file location for startup logging (spec §6.5).
// yaml:"-" prevents it being read from or written to the YAML file.
type Config struct {
	Path     string   `yaml:"-"`
	Interval int      `yaml:"interval"`
	Networks []string `yaml:"networks"`
}

// Load reads a YAML config file from path, parses and validates it,
// and returns a Config. If the file does not exist it returns
// os.ErrNotExist so the caller can trigger first-run setup.
//
// Precondition: path has already been expanded (no ~ remains).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	var cfg Config

	// Decode into a shadow struct so we can distinguish "key absent"
	// (nil pointer → default 3600, spec §4.2) from "explicitly 0"
	// (validation error per §4.3 / TestLoad_IntervalZero).
	var raw struct {
		Interval *int     `yaml:"interval"`
		Networks []string `yaml:"networks"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if raw.Interval == nil {
		cfg.Interval = 3600 // specified default (spec §4.2)
	} else {
		cfg.Interval = *raw.Interval
	}
	cfg.Networks = raw.Networks

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	cfg.Path = path

	return &cfg, nil
}

// validate checks all Config constraints. It trims whitespace from each
// network SSID (spec §4.3 "SSID whitespace: Trimmed at load time"),
// de-duplicates and warns about duplicates (spec §4.3 "Duplicate SSIDs:
// De-duplicate, print startup warning"), and returns an error for any
// violation.
func validate(cfg *Config) error {
	if cfg.Networks == nil {
		return fmt.Errorf("%w: networks key is required", ErrValidation)
	}
	// Trim whitespace at load time; reject entries that become empty.
	for i, n := range cfg.Networks {
		trimmed := strings.TrimSpace(n)
		if trimmed == "" {
			return fmt.Errorf("%w: networks entry %d is empty after trimming", ErrValidation, i)
		}
		cfg.Networks[i] = trimmed
	}
	if cfg.Interval <= 0 {
		return fmt.Errorf("%w: interval must be a positive integer, got %d", ErrValidation, cfg.Interval)
	}

	// De-duplicate networks in place, preserving order. Print a startup
	// warning if any duplicates were found.
	seen := make(map[string]struct{}, len(cfg.Networks))
	deduped := make([]string, 0, len(cfg.Networks))
	hadDuplicates := false
	for _, n := range cfg.Networks {
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			deduped = append(deduped, n)
		} else {
			hadDuplicates = true
		}
	}
	if hadDuplicates {
		log.Printf("warn: duplicate network entries removed; %d unique networks configured", len(deduped))
	}
	cfg.Networks = deduped

	return nil
}

// Contains reports whether ssid is in the configured networks list.
// The networks list is already trimmed at Load time (spec §4.3); the
// comparison here is exact and case-sensitive.
func (c *Config) Contains(ssid string) bool {
	for _, n := range c.Networks {
		if n == ssid {
			return true
		}
	}
	return false
}
