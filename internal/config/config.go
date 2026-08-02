package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the parsed antilocker.yaml contents.
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
