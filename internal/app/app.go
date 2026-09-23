// Package app contains the Agent's top-level runtime use case.
package app

import (
	"context"
	"fmt"
	log "github.com/sonar-probe/sonar-agent/internal/logging"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sonar-probe/sonar-agent/internal/collector"
	"github.com/sonar-probe/sonar-agent/internal/config"
	"github.com/sonar-probe/sonar-agent/internal/connectivity"
	"github.com/sonar-probe/sonar-agent/internal/discovery"
	"github.com/sonar-probe/sonar-agent/internal/reporter"
	"github.com/sonar-probe/sonar-agent/internal/version"
)

// Run starts the Agent runtime after command-line configuration has been
// resolved. It owns the application's lifecycle and service orchestration.
func Run(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, cfg)
}

func run(ctx context.Context, cfg config.Config) error {
	return runWithFactory(ctx, cfg, buildRuntime)
}

type runtime interface {
	Run(context.Context) error
	Close() error
}

type runtimeFactory func(context.Context, config.Config) (runtime, error)

func runWithFactory(ctx context.Context, cfg config.Config, newRuntime runtimeFactory) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	agentRuntime, err := newRuntime(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := agentRuntime.Close(); err != nil {
			log.Printf("save network traffic statistics during shutdown: %v", err)
		}
	}()

	err = agentRuntime.Run(ctx)
	if ctx.Err() != nil {
		log.Println("Agent shutdown complete")
		return nil
	}
	return err
}

type collectorRuntime interface {
	InitNetStatic()
	Close() error
	DiskList() ([]string, error)
	InterfaceList() ([]string, error)
}

type reporterRuntime interface {
	Run(context.Context) error
	RunStaticInfoReporter(context.Context)
}

type agentRuntime struct {
	collector collectorRuntime
	reporter  reporterRuntime
}

func buildRuntime(ctx context.Context, cfg config.Config) (runtime, error) {
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
			return nil, fmt.Errorf("auto-discovery failed: %w", err)
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

	log.Println("Sonar Agent", version.CurrentVersion)
	return &agentRuntime{collector: hostCollector, reporter: agentReporter}, nil
}

func (r *agentRuntime) Run(ctx context.Context) error {
	r.collector.InitNetStatic()

	diskList, err := r.collector.DiskList()
	if err != nil {
		log.Println("Failed to get disk list:", err)
	}
	log.Println("Monitoring Mountpoints:", diskList)
	interfaceList, err := r.collector.InterfaceList()
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
		r.reporter.RunStaticInfoReporter(runCtx)
	}()

	err = r.reporter.Run(runCtx)
	cancel()
	staticInfoWG.Wait()
	return err
}

func (r *agentRuntime) Close() error {
	return r.collector.Close()
}
