// Package app contains the Agent's top-level runtime use case.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/collector/netstatic"
	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/internal/discovery"
	"github.com/komari-probe/komari-probe-agent/internal/reporter"
	"github.com/komari-probe/komari-probe-agent/internal/version"
)

// Run starts the Agent runtime after command-line configuration has been
// resolved. It owns the application's lifecycle and service orchestration.
func Run(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-stopCtx.Done()
		log.Printf("shutting down gracefully...")
		if err := netstatic.Stop(); err != nil {
			log.Printf("save network traffic statistics during shutdown: %v", err)
		}
		os.Exit(0)
	}()

	if cfg.AutoDiscoveryKey != "" {
		var err error
		cfg, err = discovery.ResolveAutoDiscovery(cfg)
		if err != nil {
			return fmt.Errorf("auto-discovery failed: %w", err)
		}
	}

	hostCollector := collector.New(collector.Options{
		IncludeNICs:         cfg.IncludeNICs,
		ExcludeNICs:         cfg.ExcludeNICs,
		IncludeMountpoints:  cfg.IncludeMountpoints,
		MonthRotate:         cfg.MonthRotate,
		MemoryIncludeCache:  cfg.MemoryIncludeCache,
		MemoryReportRawUsed: cfg.MemoryReportRawUsed,
		CustomIPv4:          cfg.CustomIPv4,
		CustomIPv6:          cfg.CustomIPv6,
		GetIPAddressFromNIC: cfg.GetIPAddressFromNIC,
		HostProc:            cfg.HostProc,
	})
	agentReporter := reporter.New(reporter.Options{
		Endpoint:           cfg.Endpoint,
		Token:              cfg.Token,
		Interval:           cfg.Interval,
		MaxRetries:         cfg.MaxRetries,
		ReconnectInterval:  cfg.ReconnectInterval,
		InfoReportInterval: cfg.InfoReportInterval,
		IgnoreUnsafeCert:   cfg.IgnoreUnsafeCert,
		DisableCompression: cfg.DisableCompression,
		PreferIPVersion:    cfg.PreferIPVersion,
		EnableGPU:          cfg.EnableGPU,
	}, hostCollector)

	hostCollector.InitNetStatic()
	log.Println("Komari Agent", version.CurrentVersion)

	if cfg.CustomDNS != "" {
		connectivity.SetCustomDNSServer(cfg.CustomDNS)
		log.Printf("Using custom DNS server: %s", cfg.CustomDNS)
	} else {
		log.Printf("Using system default DNS resolver")
	}

	diskList, err := hostCollector.DiskList()
	if err != nil {
		log.Println("Failed to get disk list:", err)
	}
	log.Println("Monitoring Mountpoints:", diskList)
	interfaceList, err := hostCollector.InterfaceList()
	if err != nil {
		log.Println("Failed to get interface list:", err)
	}
	log.Println("Monitoring Interfaces:", interfaceList)

	go agentReporter.RunStaticInfoReporter()
	for {
		agentReporter.UpdateBasicInfo()
		agentReporter.EstablishWebSocketConnection()
	}
}
