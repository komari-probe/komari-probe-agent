// Package app contains the Agent's top-level runtime use case.
package app

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
	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/internal/discovery"
	"github.com/komari-probe/komari-probe-agent/internal/reporter"
	"github.com/komari-probe/komari-probe-agent/internal/version"
)

// Run starts the Agent runtime after command-line configuration has been
// resolved. It owns the application's lifecycle and service orchestration.
func Run(cfg *config.Config) error {
	if cfg.PreferIPVersion != "" && cfg.PreferIPVersion != "4" && cfg.PreferIPVersion != "6" {
		return fmt.Errorf("invalid --prefer-ip-version value %q: expected 4 or 6", cfg.PreferIPVersion)
	}

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

	if cfg.CustomDNS != "" {
		connectivity.SetCustomDNSServer(cfg.CustomDNS)
		log.Printf("Using custom DNS server: %s", cfg.CustomDNS)
	} else {
		log.Printf("Using system default DNS resolver")
	}

	if cfg.AutoDiscoveryKey != "" {
		if err := discovery.HandleAutoDiscovery(); err != nil {
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

	if cfg.IgnoreUnsafeCert {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	go reporter.DoUploadBasicInfoWorks()
	for {
		reporter.UpdateBasicInfo()
		reporter.EstablishWebSocketConnection()
	}
}
