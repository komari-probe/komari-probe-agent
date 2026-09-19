package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/collector/netstatic"
	"github.com/komari-probe/komari-probe-agent/internal/discovery"
	"github.com/komari-probe/komari-probe-agent/internal/reporter"
	"github.com/komari-probe/komari-probe-agent/internal/version"
	"github.com/komari-probe/komari-probe-agent/pkg/dnsresolver"
	"github.com/spf13/cobra"

	"github.com/komari-probe/komari-probe-agent/internal/config"
)

var flags = config.GlobalConfig

var RootCmd = &cobra.Command{
	Use:   "komari-probe-agent",
	Short: "Komari Probe Agent - Pure, lightweight, and high-precision server monitoring probe",
	Long:  `Komari Probe Agent is a secure and unprivileged server monitoring probe.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadConfiguration(cmd); err != nil {
			return err
		}

		if flags.PreferIPVersion != "" && flags.PreferIPVersion != "4" && flags.PreferIPVersion != "6" {
			return fmt.Errorf("invalid --prefer-ip-version value %q: expected 4 or 6", flags.PreferIPVersion)
		}
		// 捕获中止信号，优雅退出
		stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			<-stopCtx.Done()
			log.Printf("shutting down gracefully...")
			netstatic.Stop()
			os.Exit(0)
		}()

		collector.InitNetstatic()

		log.Println("Komari Agent", version.CurrentVersion)

		// 设置 DNS 解析行为
		if flags.CustomDNS != "" {
			dnsresolver.SetCustomDNSServer(flags.CustomDNS)
			log.Printf("Using custom DNS server: %s", flags.CustomDNS)
		} else {
			// 未设置则使用系统默认 DNS（不使用内置列表）
			log.Printf("Using system default DNS resolver")
		}

		// Auto discovery
		if flags.AutoDiscoveryKey != "" {
			err := discovery.HandleAutoDiscovery()
			if err != nil {
				return fmt.Errorf("auto-discovery failed: %w", err)
			}
		}

		diskList, err := collector.DiskList()
		if err != nil {
			log.Println("Failed to get disk list:", err)
		}
		log.Println("Monitoring Mountpoints:", diskList)
		interfaceList, err := collector.InterfaceList()
		if err != nil {
			log.Println("Failed to get interface list:", err)
		}
		log.Println("Monitoring Interfaces:", interfaceList)

		// 忽略不安全的证书
		if flags.IgnoreUnsafeCert {
			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		go reporter.DoUploadBasicInfoWorks()
		for {
			reporter.UpdateBasicInfo()
			reporter.EstablishWebSocketConnection()
		}
	},
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

	if err := RootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

// loadConfiguration resolves values using the conventional precedence order:
// built-in defaults < JSON file < AGENT_* environment variables < explicit CLI flags.
func loadConfiguration(cmd *cobra.Command) error {
	cliValues := *flags
	configPath := configFilePath(cmd, cliValues)

	*flags = config.Default()
	if configPath != "" {
		if err := config.LoadFile(configPath, flags); err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}
	}
	if err := config.ApplyEnvironment(flags, os.LookupEnv); err != nil {
		return fmt.Errorf("load environment configuration: %w", err)
	}

	applyCLIOverrides(cmd, cliValues)
	// config_file selects the source to load; a value inside the file must not
	// redirect loading after the fact.
	flags.ConfigFile = configPath
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

func applyCLIOverrides(cmd *cobra.Command, cliValues config.Config) {
	if flagChanged(cmd, "token") {
		flags.Token = cliValues.Token
	}
	if flagChanged(cmd, "endpoint") {
		flags.Endpoint = cliValues.Endpoint
	}
	if flagChanged(cmd, "auto-discovery") {
		flags.AutoDiscoveryKey = cliValues.AutoDiscoveryKey
	}
	if flagChanged(cmd, "interval") {
		flags.Interval = cliValues.Interval
	}
	if flagChanged(cmd, "ignore-unsafe-cert") {
		flags.IgnoreUnsafeCert = cliValues.IgnoreUnsafeCert
	}
	if flagChanged(cmd, "max-retries") {
		flags.MaxRetries = cliValues.MaxRetries
	}
	if flagChanged(cmd, "reconnect-interval") {
		flags.ReconnectInterval = cliValues.ReconnectInterval
	}
	if flagChanged(cmd, "info-report-interval") {
		flags.InfoReportInterval = cliValues.InfoReportInterval
	}
	if flagChanged(cmd, "include-nics") {
		flags.IncludeNics = cliValues.IncludeNics
	}
	if flagChanged(cmd, "exclude-nics") {
		flags.ExcludeNics = cliValues.ExcludeNics
	}
	if flagChanged(cmd, "include-mountpoint") {
		flags.IncludeMountpoints = cliValues.IncludeMountpoints
	}
	if flagChanged(cmd, "month-rotate") {
		flags.MonthRotate = cliValues.MonthRotate
	}
	if flagChanged(cmd, "memory-include-cache") {
		flags.MemoryIncludeCache = cliValues.MemoryIncludeCache
	}
	if flagChanged(cmd, "memory-exclude-bcf") {
		flags.MemoryReportRawUsed = cliValues.MemoryReportRawUsed
	}
	if flagChanged(cmd, "custom-dns") {
		flags.CustomDNS = cliValues.CustomDNS
	}
	if flagChanged(cmd, "gpu") {
		flags.EnableGPU = cliValues.EnableGPU
	}
	if flagChanged(cmd, "custom-ipv4") {
		flags.CustomIpv4 = cliValues.CustomIpv4
	}
	if flagChanged(cmd, "custom-ipv6") {
		flags.CustomIpv6 = cliValues.CustomIpv6
	}
	if flagChanged(cmd, "get-ip-addr-from-nic") {
		flags.GetIpAddrFromNic = cliValues.GetIpAddrFromNic
	}
	if flagChanged(cmd, "disable-compression") {
		flags.DisableCompression = cliValues.DisableCompression
	}
	if flagChanged(cmd, "prefer-ip-version") {
		flags.PreferIPVersion = cliValues.PreferIPVersion
	}
}

func flagChanged(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Changed(name) || cmd.PersistentFlags().Changed(name)
}

func init() {
	defaults := config.Default()
	RootCmd.PersistentFlags().StringVarP(&flags.Token, "token", "t", "", "API token")
	RootCmd.PersistentFlags().StringVarP(&flags.Endpoint, "endpoint", "e", "", "API endpoint")
	RootCmd.PersistentFlags().StringVar(&flags.AutoDiscoveryKey, "auto-discovery", "", "Auto discovery key for the agent")
	RootCmd.PersistentFlags().Float64VarP(&flags.Interval, "interval", "i", defaults.Interval, "Interval in seconds")
	RootCmd.PersistentFlags().BoolVarP(&flags.IgnoreUnsafeCert, "ignore-unsafe-cert", "u", false, "Ignore unsafe certificate errors")
	RootCmd.PersistentFlags().IntVarP(&flags.MaxRetries, "max-retries", "r", defaults.MaxRetries, "Maximum number of retries")
	RootCmd.PersistentFlags().IntVarP(&flags.ReconnectInterval, "reconnect-interval", "c", defaults.ReconnectInterval, "Reconnect interval in seconds")
	RootCmd.PersistentFlags().IntVar(&flags.InfoReportInterval, "info-report-interval", defaults.InfoReportInterval, "Interval in minutes for reporting basic info")
	RootCmd.PersistentFlags().StringVar(&flags.IncludeNics, "include-nics", "", "Comma-separated list of network interfaces to include")
	RootCmd.PersistentFlags().StringVar(&flags.ExcludeNics, "exclude-nics", "", "Comma-separated list of network interfaces to exclude")
	RootCmd.PersistentFlags().StringVar(&flags.IncludeMountpoints, "include-mountpoint", "", "Semicolon-separated list of mount points to include for disk statistics")
	RootCmd.PersistentFlags().IntVar(&flags.MonthRotate, "month-rotate", 0, "Month reset for network statistics (0 to disable)")
	RootCmd.PersistentFlags().BoolVar(&flags.MemoryIncludeCache, "memory-include-cache", false, "Include cache/buffer in memory usage")
	RootCmd.PersistentFlags().BoolVar(&flags.MemoryReportRawUsed, "memory-exclude-bcf", false, "Use \"raminfo.Used = v.Total - v.Free - v.Buffers - v.Cached\" calculation for memory usage")
	RootCmd.PersistentFlags().StringVar(&flags.CustomDNS, "custom-dns", "", "Custom DNS server to use (e.g. 8.8.8.8, 114.114.114.114). By default, the program uses the system DNS resolver.")
	RootCmd.PersistentFlags().BoolVar(&flags.EnableGPU, "gpu", false, "Enable detailed GPU monitoring (usage, memory, multi-GPU support)")
	RootCmd.PersistentFlags().StringVar(&flags.CustomIpv4, "custom-ipv4", "", "Custom IPv4 address to use")
	RootCmd.PersistentFlags().StringVar(&flags.CustomIpv6, "custom-ipv6", "", "Custom IPv6 address to use")
	RootCmd.PersistentFlags().BoolVar(&flags.GetIpAddrFromNic, "get-ip-addr-from-nic", false, "Get IP address from network interface")
	RootCmd.PersistentFlags().StringVar(&flags.ConfigFile, "config", "", "Path to the configuration file")
	RootCmd.PersistentFlags().BoolVar(&flags.DisableCompression, "disable-compression", false, "Disable v2 gzip/permessage-deflate compression")
	RootCmd.PersistentFlags().StringVar(&flags.PreferIPVersion, "prefer-ip-version", "", "Prefer IP version for dashboard connections: 4 or 6")
	RootCmd.PersistentFlags().ParseErrorsWhitelist.UnknownFlags = true
}
