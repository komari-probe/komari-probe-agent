package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/spf13/cobra"
)

func TestLoadConfigurationPrecedence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(configPath, []byte(`{
  "token": "from-file",
  "interval": 10,
  "reconnect_interval": 9
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_TOKEN", "from-environment")
	t.Setenv("AGENT_INTERVAL", "20")

	cliValues := config.Default()
	command := newConfigurationCommand(&cliValues)
	if err := command.ParseFlags([]string{"--config", configPath, "--token", "from-cli", "--interval", "30"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	if err := loadConfiguration(command, &cliValues); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}

	if cliValues.Token != "from-cli" {
		t.Errorf("Token = %q, want CLI value", cliValues.Token)
	}
	if cliValues.Interval != 30 {
		t.Errorf("Interval = %v, want CLI value", cliValues.Interval)
	}
	if cliValues.ReconnectInterval != 9 {
		t.Errorf("ReconnectInterval = %d, want file value", cliValues.ReconnectInterval)
	}
	if cliValues.ConfigFile != configPath {
		t.Errorf("ConfigFile = %q, want %q", cliValues.ConfigFile, configPath)
	}
}

func TestEnvironmentSelectsConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(configPath, []byte(`{"interval": 12}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_CONFIG_FILE", configPath)

	cliValues := config.Default()
	command := newConfigurationCommand(&cliValues)
	if err := loadConfiguration(command, &cliValues); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
	if cliValues.Interval != 12 {
		t.Errorf("Interval = %v, want file value", cliValues.Interval)
	}
	if cliValues.ConfigFile != configPath {
		t.Errorf("ConfigFile = %q, want %q", cliValues.ConfigFile, configPath)
	}
}

func TestCLIConfigFileOverridesEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	environmentConfigPath := filepath.Join(tempDir, "environment.json")
	cliConfigPath := filepath.Join(tempDir, "cli.json")
	if err := os.WriteFile(environmentConfigPath, []byte(`{"interval": 12}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliConfigPath, []byte(`{"interval": 24}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_CONFIG_FILE", environmentConfigPath)

	cliValues := config.Default()
	command := newConfigurationCommand(&cliValues)
	if err := command.ParseFlags([]string{"--config", cliConfigPath}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if err := loadConfiguration(command, &cliValues); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
	if cliValues.Interval != 24 || cliValues.ConfigFile != cliConfigPath {
		t.Fatalf("configuration = interval %v, file %q; want 24, %q", cliValues.Interval, cliValues.ConfigFile, cliConfigPath)
	}
}

func TestExplicitFalseFlagOverridesEnvironment(t *testing.T) {
	t.Setenv("AGENT_IGNORE_UNSAFE_CERT", "true")
	cliValues := config.Default()
	command := newConfigurationCommand(&cliValues)
	if err := command.ParseFlags([]string{"--ignore-unsafe-cert=false"}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if err := loadConfiguration(command, &cliValues); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
	if cliValues.IgnoreUnsafeCert {
		t.Fatal("explicit false CLI flag did not override environment true")
	}
}

func TestConfigFileDoesNotRedirectItsOwnSource(t *testing.T) {
	tempDir := t.TempDir()
	redirectedPath := filepath.Join(tempDir, "redirected.json")
	configPath := filepath.Join(tempDir, "agent.json")
	if err := os.WriteFile(redirectedPath, []byte(`{"interval": 99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	contents, err := json.Marshal(map[string]any{"config_file": redirectedPath, "interval": 13})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	cliValues := config.Default()
	command := newConfigurationCommand(&cliValues)
	if err := command.ParseFlags([]string{"--config", configPath}); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if err := loadConfiguration(command, &cliValues); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
	if cliValues.Interval != 13 || cliValues.ConfigFile != configPath {
		t.Fatalf("configuration = interval %v, file %q; want 13, %q", cliValues.Interval, cliValues.ConfigFile, configPath)
	}
}

func newConfigurationCommand(values *config.Config) *cobra.Command {
	command := &cobra.Command{}
	command.Flags().StringVarP(&values.Token, "token", "t", values.Token, "")
	command.Flags().Float64VarP(&values.Interval, "interval", "i", values.Interval, "")
	command.Flags().IntVarP(&values.ReconnectInterval, "reconnect-interval", "c", values.ReconnectInterval, "")
	command.Flags().BoolVarP(&values.IgnoreUnsafeCert, "ignore-unsafe-cert", "u", values.IgnoreUnsafeCert, "")
	command.Flags().StringVar(&values.ConfigFile, "config", values.ConfigFile, "")
	return command
}
