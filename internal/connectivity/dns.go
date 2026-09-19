package connectivity

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

var (
	dnsServers = []string{
		"[2606:4700:4700::1111]:53",
		"[2606:4700:4700::1001]:53",
		"[2001:4860:4860::8888]:53",
		"[2001:4860:4860::8844]:53",
		"114.114.114.114:53",
		"1.1.1.1:53",
		"8.8.8.8:53",
		"8.8.4.4:53",
		"223.5.5.5:53",
		"119.29.29.29:53",
	}
	customDNSServer string
)

// SetCustomDNSServer configures the DNS server used for outbound connections.
func SetCustomDNSServer(dnsServer string) {
	if dnsServer == "" {
		return
	}
	customDNSServer = normalizeDNSServer(dnsServer)
}

func normalizeDNSServer(server string) string {
	server = strings.TrimSpace(server)
	if (strings.HasPrefix(server, "[") && strings.Contains(server, "]:")) ||
		(strings.Count(server, ":") == 1 && !strings.Contains(server, "]")) {
		return server
	}
	if strings.Count(server, ":") >= 2 && !strings.Contains(server, "]") {
		return "[" + server + "]:53"
	}
	if !strings.Contains(server, ":") {
		return server + ":53"
	}
	return server
}

func currentDNSServer() string {
	return customDNSServer
}

func customResolver() *net.Resolver {
	if currentDNSServer() == "" {
		return net.DefaultResolver
	}

	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: 10 * time.Second}
			server := currentDNSServer()
			if conn, err := dialer.DialContext(ctx, "udp", server); err == nil {
				return conn, nil
			}

			log.Printf("Custom DNS server %s is unreachable, trying fallback servers", server)
			for _, fallback := range dnsServers {
				if fallback == server {
					continue
				}
				if conn, err := dialer.DialContext(ctx, "udp", fallback); err == nil {
					return conn, nil
				}
			}
			return nil, fmt.Errorf("no available DNS server")
		},
	}
}
