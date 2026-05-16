package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	toml "github.com/pelletier/go-toml/v2"
)

type fileConfig struct {
	Dolt map[string]any `toml:"dolt"`
}

// ConfigPath returns the user-level sc config path.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".sc", "config.toml"), nil
}

// LoadFileConfig reads ~/.sc/config.toml if present.
func (c *Config) LoadFileConfig() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is constructed from os.UserHomeDir() + fixed suffix, not user input.
	if errors.Is(err, os.ErrNotExist) {
		c.fileValues = map[string]string{}
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var parsed fileConfig
	if err := toml.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	values := map[string]string{}
	for key, value := range parsed.Dolt {
		fullKey := "dolt." + key
		if !IsKnownKey(fullKey) {
			continue
		}
		values[fullKey] = valueToString(value)
	}
	c.fileValues = values
	return nil
}

// Get returns a string config value using explicit CLI flag > env > file > default precedence.
func (c *Config) Get(key, defaultVal string) string {
	value, _ := c.ResolveValue(key, defaultVal)
	return value
}

// GetInt returns an int config value using Get's precedence.
func (c *Config) GetInt(key string, defaultVal int) int {
	value := c.Get(key, "")
	if value == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultVal
	}
	return parsed
}

// SetFileValue writes one supported config key to ~/.sc/config.toml.
func SetFileValue(key, value string) (string, error) {
	if !IsKnownKey(key) {
		return "", fmt.Errorf("unknown config key %q", key)
	}
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}

	cfg := fileConfig{Dolt: map[string]any{}}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is constructed from os.UserHomeDir() + fixed suffix, not user input.
	if err == nil && len(data) > 0 {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return "", fmt.Errorf("parsing %s: %w", path, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	if cfg.Dolt == nil {
		cfg.Dolt = map[string]any{}
	}

	shortKey := key[len("dolt."):]
	if key == KeyDoltTimeout {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.Dolt[shortKey] = parsed
		} else {
			cfg.Dolt[shortKey] = value
		}
	} else {
		cfg.Dolt[shortKey] = value
	}

	encoded, err := toml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("encoding config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

func valueToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}
