package cmd

import (
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

	originalFlags := flags
	flags = &cliValues
	t.Cleanup(func() { flags = originalFlags })

	if err := loadConfiguration(command); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}

	if flags.Token != "from-cli" {
		t.Errorf("Token = %q, want CLI value", flags.Token)
	}
	if flags.Interval != 30 {
		t.Errorf("Interval = %v, want CLI value", flags.Interval)
	}
	if flags.ReconnectInterval != 9 {
		t.Errorf("ReconnectInterval = %d, want file value", flags.ReconnectInterval)
	}
	if flags.ConfigFile != configPath {
		t.Errorf("ConfigFile = %q, want %q", flags.ConfigFile, configPath)
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
	originalFlags := flags
	flags = &cliValues
	t.Cleanup(func() { flags = originalFlags })

	if err := loadConfiguration(command); err != nil {
		t.Fatalf("loadConfiguration() error = %v", err)
	}
	if flags.Interval != 12 {
		t.Errorf("Interval = %v, want file value", flags.Interval)
	}
	if flags.ConfigFile != configPath {
		t.Errorf("ConfigFile = %q, want %q", flags.ConfigFile, configPath)
	}
}

func newConfigurationCommand(values *config.Config) *cobra.Command {
	command := &cobra.Command{}
	command.Flags().StringVarP(&values.Token, "token", "t", values.Token, "")
	command.Flags().Float64VarP(&values.Interval, "interval", "i", values.Interval, "")
	command.Flags().IntVarP(&values.ReconnectInterval, "reconnect-interval", "c", values.ReconnectInterval, "")
	command.Flags().StringVar(&values.ConfigFile, "config", values.ConfigFile, "")
	return command
}
