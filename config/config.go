package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	configDir  = "irons"
	configFile = "config.yml"
)

// Config holds the persistent CLI configuration.
type Config struct {
	APIKey string `yaml:"api_key,omitempty"`
}

// configPath returns the path to the config file:
// $XDG_CONFIG_HOME/irons/config.yml or ~/.config/irons/config.yml
func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, configDir, configFile), nil
}

// Load reads the config file and returns a Config. If the file does not exist
// an empty Config is returned without error.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the Config to disk, creating any necessary parent directories.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// SetAPIKey is a convenience helper that loads the existing config, sets the
// API key, and saves it back.
func SetAPIKey(token string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.APIKey = token
	return Save(cfg)
}
