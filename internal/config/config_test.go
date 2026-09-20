package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyEnvironmentOverlaysValues(t *testing.T) {
	cfg := Config{Interval: 3, MaxRetries: 3, IgnoreUnsafeCert: false, Token: "from-file"}
	env := map[string]string{
		"AGENT_TOKEN":              "from-environment",
		"AGENT_INTERVAL":           "8.5",
		"AGENT_MAX_RETRIES":        "7",
		"AGENT_IGNORE_UNSAFE_CERT": "true",
	}

	err := ApplyEnvironment(&cfg, func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	})
	if err != nil {
		t.Fatalf("ApplyEnvironment() error = %v", err)
	}

	if cfg.Token != "from-environment" || cfg.Interval != 8.5 || cfg.MaxRetries != 7 || !cfg.IgnoreUnsafeCert {
		t.Fatalf("environment values were not applied: %#v", cfg)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{name: "valid token", cfg: Config{Endpoint: "https://example.com", Token: "token", Interval: 1, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1}},
		{name: "valid auto discovery", cfg: Config{Endpoint: "https://example.com", AutoDiscoveryKey: "key", Interval: 1, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1}},
		{name: "missing endpoint", cfg: Config{Token: "token", Interval: 1, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1}, want: true},
		{name: "invalid endpoint", cfg: Config{Endpoint: "example.com", Token: "token", Interval: 1, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1}, want: true},
		{name: "missing credentials", cfg: Config{Endpoint: "https://example.com", Interval: 1, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1}, want: true},
		{name: "zero interval", cfg: Config{Endpoint: "https://example.com", Token: "token", Interval: 0, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1}, want: true},
		{name: "negative retries", cfg: Config{Endpoint: "https://example.com", Token: "token", Interval: 1, MaxRetries: -1, ReconnectInterval: 1, InfoReportInterval: 1}, want: true},
		{name: "invalid IP preference", cfg: Config{Endpoint: "https://example.com", Token: "token", Interval: 1, MaxRetries: 0, ReconnectInterval: 1, InfoReportInterval: 1, PreferIPVersion: "5"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Validate(); (got != nil) != tt.want {
				t.Fatalf("Validate() error = %v, want error = %t", got, tt.want)
			}
		})
	}
}

func TestLoadFileRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"endpoint":"https://example.com","token":"token","typo":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadFile(path, &Config{}); err == nil {
		t.Fatal("LoadFile() accepted an unknown field")
	}
}

func TestApplyEnvironmentRejectsInvalidValues(t *testing.T) {
	cfg := Default()
	err := ApplyEnvironment(&cfg, func(name string) (string, bool) {
		if name == "AGENT_INTERVAL" {
			return "not-a-number", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("ApplyEnvironment() succeeded with an invalid number")
	}
}
