package connectivity

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	preferV4Once sync.Once
	hasIPv4      bool
)

// GetNetDialer returns a dialer that uses the configured DNS resolver.
func GetNetDialer(timeout time.Duration) *net.Dialer {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second, Resolver: customResolver()}
}

// GetDialContextWithPreference resolves a host, sorts addresses by the
// requested IP-version preference, and attempts each address in turn.
func GetDialContextWithPreference(timeout time.Duration, preferIPVersion string) func(context.Context, string, string) (net.Conn, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	resolver := customResolver()
	preferIPVersion = normalizeIPVersionPreference(preferIPVersion)

	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		lookupCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ips, err := resolver.LookupHost(lookupCtx, host)
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
	}
}

func normalizeIPVersionPreference(preferIPVersion string) string {
	if preferIPVersion == "4" || preferIPVersion == "6" {
		return preferIPVersion
	}
	return ""
}

func sortIPsByPreference(ips []string, preferIPVersion string) {
	if preferIPVersion == "" {
		if preferIPv4First() {
			preferIPVersion = "4"
		} else {
			preferIPVersion = "6"
		}
	}

	preferred := make([]string, 0, len(ips))
	others := make([]string, 0, len(ips))
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		isIPv4 := parsed != nil && parsed.To4() != nil
		isIPv6 := parsed != nil && parsed.To4() == nil
		if (preferIPVersion == "4" && isIPv4) || (preferIPVersion == "6" && isIPv6) {
			preferred = append(preferred, ip)
		} else {
			others = append(others, ip)
		}
	}
	copy(ips, append(preferred, others...))
}

func preferIPv4First() bool {
	preferV4Once.Do(func() {
		interfaces, _ := net.Interfaces()
		for _, iface := range interfaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			addresses, _ := iface.Addrs()
			for _, address := range addresses {
				var ip net.IP
				switch value := address.(type) {
				case *net.IPNet:
					ip = value.IP
				case *net.IPAddr:
					ip = value.IP
				}
				if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
					hasIPv4 = true
					return
				}
			}
		}
	})
	return hasIPv4
}
