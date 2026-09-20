package cmd

import (
	"fmt"
	log "github.com/komari-probe/komari-probe-agent/internal/logging"
	"os"

	"github.com/komari-probe/komari-probe-agent/internal/app"
	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/spf13/cobra"
)

// NewRootCmd creates an isolated command and configuration value for one
// invocation. No configuration is shared across executions or tests.
func NewRootCmd() *cobra.Command {
	cfg := config.Default()
	root := &cobra.Command{
		Use:   "komari-probe-agent",
		Short: "Komari Probe Agent - Pure, lightweight, and high-precision server monitoring probe",
		Long:  `Komari Probe Agent is a secure and unprivileged server monitoring probe.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := loadConfiguration(cmd, &cfg); err != nil {
				return err
			}
			return log.SetLevel(cfg.LogLevel)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return app.Run(cfg)
		},
	}
	bindPersistentFlags(root, &cfg)
	root.AddCommand(newCheckMemCmd(&cfg), newListDiskCmd(&cfg))
	return root
}

func handleDeprecatedFlags() {
	var filtered []string
	for _, arg := range os.Args {
		if arg == "-memory-mode-available" || arg == "--memory-mode-available" {
			log.Println("WARNING: The --memory-mode-available flag is deprecated in version 1.0.70 and later. Use --memory-include-cache to report memory usage including cache/buffer.")
			continue
		}
		filtered = append(filtered, arg)
	}
	os.Args = filtered
}

func Execute() {
	handleDeprecatedFlags()
	if err := NewRootCmd().Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// loadConfiguration resolves values using the conventional precedence order:
// built-in defaults < JSON file < AGENT_* environment variables < explicit CLI flags.
func loadConfiguration(cmd *cobra.Command, dst *config.Config) error {
	cliValues := *dst
	configPath := configFilePath(cmd, cliValues)

	*dst = config.Default()
	if configPath != "" {
		if err := config.LoadFile(configPath, dst); err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}
	}
	if err := config.ApplyEnvironment(dst, os.LookupEnv); err != nil {
		return fmt.Errorf("load environment configuration: %w", err)
	}

	applyCLIOverrides(cmd, cliValues, dst)
	// config_file selects the source to load; a value inside the file must not
	// redirect loading after the fact.
	dst.ConfigFile = configPath
	return nil
}

func configFilePath(cmd *cobra.Command, cliValues config.Config) string {
	if flagChanged(cmd, "config") {
		return cliValues.ConfigFile
	}
	if path, ok := os.LookupEnv("AGENT_CONFIG_FILE"); ok {
		return path
	}
	return ""
}

func applyCLIOverrides(cmd *cobra.Command, cliValues config.Config, dst *config.Config) {
	if flagChanged(cmd, "token") {
		dst.Token = cliValues.Token
	}
	if flagChanged(cmd, "endpoint") {
		dst.Endpoint = cliValues.Endpoint
	}
	if flagChanged(cmd, "auto-discovery") {
		dst.AutoDiscoveryKey = cliValues.AutoDiscoveryKey
	}
	if flagChanged(cmd, "interval") {
		dst.Interval = cliValues.Interval
	}
	if flagChanged(cmd, "ignore-unsafe-cert") {
		dst.IgnoreUnsafeCert = cliValues.IgnoreUnsafeCert
	}
	if flagChanged(cmd, "max-retries") {
		dst.MaxRetries = cliValues.MaxRetries
	}
	if flagChanged(cmd, "reconnect-interval") {
		dst.ReconnectInterval = cliValues.ReconnectInterval
	}
	if flagChanged(cmd, "info-report-interval") {
		dst.InfoReportInterval = cliValues.InfoReportInterval
	}
	if flagChanged(cmd, "include-nics") {
		dst.IncludeNICs = cliValues.IncludeNICs
	}
	if flagChanged(cmd, "exclude-nics") {
		dst.ExcludeNICs = cliValues.ExcludeNICs
	}
	if flagChanged(cmd, "include-mountpoint") {
		dst.IncludeMountpoints = cliValues.IncludeMountpoints
	}
	if flagChanged(cmd, "month-rotate") {
		dst.MonthRotate = cliValues.MonthRotate
	}
	if flagChanged(cmd, "memory-include-cache") {
		dst.MemoryIncludeCache = cliValues.MemoryIncludeCache
	}
	if flagChanged(cmd, "memory-exclude-bcf") {
		dst.MemoryReportRawUsed = cliValues.MemoryReportRawUsed
	}
	if flagChanged(cmd, "custom-dns") {
		dst.CustomDNS = cliValues.CustomDNS
	}
	if flagChanged(cmd, "gpu") {
		dst.EnableGPU = cliValues.EnableGPU
	}
	if flagChanged(cmd, "custom-ipv4") {
		dst.CustomIPv4 = cliValues.CustomIPv4
	}
	if flagChanged(cmd, "custom-ipv6") {
		dst.CustomIPv6 = cliValues.CustomIPv6
	}
	if flagChanged(cmd, "get-ip-addr-from-nic") {
		dst.GetIPAddressFromNIC = cliValues.GetIPAddressFromNIC
	}
	if flagChanged(cmd, "disable-compression") {
		dst.DisableCompression = cliValues.DisableCompression
	}
	if flagChanged(cmd, "prefer-ip-version") {
		dst.PreferIPVersion = cliValues.PreferIPVersion
	}
	if flagChanged(cmd, "log-level") {
		dst.LogLevel = cliValues.LogLevel
	}
}

func flagChanged(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Changed(name) || cmd.PersistentFlags().Changed(name)
}

func bindPersistentFlags(command *cobra.Command, cfg *config.Config) {
	defaults := config.Default()
	command.PersistentFlags().StringVarP(&cfg.Token, "token", "t", "", "API token")
	command.PersistentFlags().StringVarP(&cfg.Endpoint, "endpoint", "e", "", "API endpoint")
	command.PersistentFlags().StringVar(&cfg.AutoDiscoveryKey, "auto-discovery", "", "Auto discovery key for the agent")
	command.PersistentFlags().Float64VarP(&cfg.Interval, "interval", "i", defaults.Interval, "Interval in seconds")
	command.PersistentFlags().BoolVarP(&cfg.IgnoreUnsafeCert, "ignore-unsafe-cert", "u", false, "Ignore unsafe certificate errors")
	command.PersistentFlags().IntVarP(&cfg.MaxRetries, "max-retries", "r", defaults.MaxRetries, "Maximum number of retries")
	command.PersistentFlags().IntVarP(&cfg.ReconnectInterval, "reconnect-interval", "c", defaults.ReconnectInterval, "Reconnect interval in seconds")
	command.PersistentFlags().IntVar(&cfg.InfoReportInterval, "info-report-interval", defaults.InfoReportInterval, "Interval in minutes for reporting basic info")
	command.PersistentFlags().StringVar(&cfg.IncludeNICs, "include-nics", "", "Comma-separated list of network interfaces to include")
	command.PersistentFlags().StringVar(&cfg.ExcludeNICs, "exclude-nics", "", "Comma-separated list of network interfaces to exclude")
	command.PersistentFlags().StringVar(&cfg.IncludeMountpoints, "include-mountpoint", "", "Semicolon-separated list of mount points to include for disk statistics")
	command.PersistentFlags().IntVar(&cfg.MonthRotate, "month-rotate", 0, "Month reset for network statistics (0 to disable)")
	command.PersistentFlags().BoolVar(&cfg.MemoryIncludeCache, "memory-include-cache", false, "Include cache/buffer in memory usage")
	command.PersistentFlags().BoolVar(&cfg.MemoryReportRawUsed, "memory-exclude-bcf", false, "Use \"raminfo.Used = v.Total - v.Free - v.Buffers - v.Cached\" calculation for memory usage")
	command.PersistentFlags().StringVar(&cfg.CustomDNS, "custom-dns", "", "Custom DNS server to use (e.g. 8.8.8.8, 114.114.114.114). By default, the program uses the system DNS resolver.")
	command.PersistentFlags().BoolVar(&cfg.EnableGPU, "gpu", false, "Enable detailed GPU monitoring (usage, memory, multi-GPU support)")
	command.PersistentFlags().StringVar(&cfg.CustomIPv4, "custom-ipv4", "", "Custom IPv4 address to use")
	command.PersistentFlags().StringVar(&cfg.CustomIPv6, "custom-ipv6", "", "Custom IPv6 address to use")
	command.PersistentFlags().BoolVar(&cfg.GetIPAddressFromNIC, "get-ip-addr-from-nic", false, "Get IP address from network interface")
	command.PersistentFlags().StringVar(&cfg.ConfigFile, "config", "", "Path to the configuration file")
	command.PersistentFlags().BoolVar(&cfg.DisableCompression, "disable-compression", false, "Disable v2 gzip/permessage-deflate compression")
	command.PersistentFlags().StringVar(&cfg.PreferIPVersion, "prefer-ip-version", "", "Prefer IP version for dashboard connections: 4 or 6")
	command.PersistentFlags().StringVar(&cfg.LogLevel, "log-level", defaults.LogLevel, "Minimum log level: debug, info, warn, or error")
	command.PersistentFlags().ParseErrorsWhitelist.UnknownFlags = true
}
