package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("parse config file: expected a single JSON object")
	}
	return nil
}
