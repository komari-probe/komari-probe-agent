package connectivity

import (
	"net"
	"net/http"
	"sync"
)

// Options configures outbound network connectivity for one Agent run.
type Options struct {
	CustomDNSServer string
}

// Manager owns DNS resolution, connection preferences, and cached HTTP
// clients for one Agent run.
type Manager struct {
	dnsResolver *net.Resolver

	preferV4Once sync.Once
	hasIPv4      bool

	httpClientMu sync.Mutex
	httpClients  map[httpClientKey]*http.Client
}

// NewManager creates an isolated outbound-connectivity manager.
func NewManager(options Options) *Manager {
	server := normalizeDNSServer(options.CustomDNSServer)
	return &Manager{
		dnsResolver: newResolver(server),
		httpClients: make(map[httpClientKey]*http.Client),
	}
}
