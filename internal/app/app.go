// Package app contains the Agent's top-level runtime use case.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/internal/discovery"
	"github.com/komari-probe/komari-probe-agent/internal/reporter"
	"github.com/komari-probe/komari-probe-agent/internal/version"
)

// Run starts the Agent runtime after command-line configuration has been
// resolved. It owns the application's lifecycle and service orchestration.
func Run(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, cfg)
}

func run(ctx context.Context, cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	connections := connectivity.NewManager(connectivity.Options{CustomDNSServer: cfg.CustomDNS})
	if cfg.CustomDNS != "" {
		log.Printf("Using custom DNS server: %s", cfg.CustomDNS)
	} else {
		log.Println("Using system default DNS resolver")
	}

	if cfg.AutoDiscoveryKey != "" {
		var err error
		cfg, err = discovery.ResolveAutoDiscovery(ctx, cfg, connections)
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
		Connectivity:        connections,
		IgnoreUnsafeCert:    cfg.IgnoreUnsafeCert,
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
	}, hostCollector, connections)

	hostCollector.InitNetStatic()
	defer func() {
		if err := hostCollector.Close(); err != nil {
			log.Printf("save network traffic statistics during shutdown: %v", err)
		}
	}()
	log.Println("Komari Agent", version.CurrentVersion)

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

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var staticInfoWG sync.WaitGroup
	staticInfoWG.Add(1)
	go func() {
		defer staticInfoWG.Done()
		agentReporter.RunStaticInfoReporter(runCtx)
	}()

	err = agentReporter.Run(runCtx)
	cancel()
	staticInfoWG.Wait()
	if ctx.Err() != nil {
		log.Println("Agent shutdown complete")
		return nil
	}
	return err
}
