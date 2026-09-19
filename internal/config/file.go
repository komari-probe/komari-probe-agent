package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile overlays the JSON configuration at path onto dst. Callers should
// initialise dst with Default before loading a file so omitted fields retain
// their defaults.
func LoadFile(path string, dst *Config) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := json.Unmarshal(contents, dst); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	return nil
}
