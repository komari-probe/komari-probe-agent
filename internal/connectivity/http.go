package connectivity

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

type httpClientKey struct {
	timeout          time.Duration
	ignoreUnsafeCert bool
	preferIPVersion  string
}

// NewHTTPClientWithPreference returns a cached client using the configured
// resolver and the requested IP-version preference.
func (manager *Manager) NewHTTPClientWithPreference(timeout time.Duration, preferIPVersion string, ignoreUnsafeCert bool) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	preferIPVersion = normalizeIPVersionPreference(preferIPVersion)
	key := httpClientKey{timeout: timeout, ignoreUnsafeCert: ignoreUnsafeCert, preferIPVersion: preferIPVersion}

	manager.httpClientMu.Lock()
	defer manager.httpClientMu.Unlock()
	if client := manager.httpClients[key]; client != nil {
		return client
	}
	client := &http.Client{
		Transport: manager.buildTransport(timeout, &tls.Config{InsecureSkipVerify: ignoreUnsafeCert}, preferIPVersion),
		Timeout:   timeout,
	}
	manager.httpClients[key] = client
	return client
}

func (manager *Manager) buildTransport(timeout time.Duration, tlsConfig *tls.Config, preferIPVersion string) *http.Transport {
	resolver := manager.dnsResolver
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolver.LookupHost(ctx, host)
			if err != nil {
				return nil, err
			}
			manager.sortIPsByPreference(ips, preferIPVersion)
			for _, ip := range ips {
				dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second, DualStack: true}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
				if err == nil {
					return conn, nil
				}
			}
			return nil, fmt.Errorf("failed to dial to any of the resolved IPs")
		},
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
	}
}
