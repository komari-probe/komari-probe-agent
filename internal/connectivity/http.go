package connectivity

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type httpClientKey struct {
	timeout          time.Duration
	ignoreUnsafeCert bool
	preferIPVersion  string
}

var (
	httpClientMu sync.Mutex
	httpClients  = make(map[httpClientKey]*http.Client)
)

// GetHTTPClientWithPreference returns a cached client using the configured
// resolver and the requested IP-version preference.
func GetHTTPClientWithPreference(timeout time.Duration, preferIPVersion string, ignoreUnsafeCert bool) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	preferIPVersion = normalizeIPVersionPreference(preferIPVersion)
	key := httpClientKey{timeout: timeout, ignoreUnsafeCert: ignoreUnsafeCert, preferIPVersion: preferIPVersion}

	httpClientMu.Lock()
	defer httpClientMu.Unlock()
	if client := httpClients[key]; client != nil {
		return client
	}
	client := &http.Client{
		Transport: buildTransport(timeout, &tls.Config{InsecureSkipVerify: ignoreUnsafeCert}, preferIPVersion),
		Timeout:   timeout,
	}
	httpClients[key] = client
	return client
}

func buildTransport(timeout time.Duration, tlsConfig *tls.Config, preferIPVersion string) *http.Transport {
	resolver := customResolver()
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
			sortIPsByPreference(ips, preferIPVersion)
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
