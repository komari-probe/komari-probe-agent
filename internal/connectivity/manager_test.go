package connectivity

import (
	"net"
	"testing"
	"time"
)

func TestManagerOwnsConnectionState(t *testing.T) {
	systemManager := NewManager(Options{})
	customManager := NewManager(Options{CustomDNSServer: "1.1.1.1"})

	if systemManager.dnsResolver != net.DefaultResolver {
		t.Fatal("system manager does not use the default resolver")
	}
	if customManager.dnsResolver == net.DefaultResolver {
		t.Fatal("custom manager does not use its own resolver")
	}

	firstClient := customManager.NewHTTPClientWithPreference(time.Second, "4", false)
	if again := customManager.NewHTTPClientWithPreference(time.Second, "4", false); again != firstClient {
		t.Fatal("manager did not reuse its HTTP client")
	}
	if other := systemManager.NewHTTPClientWithPreference(time.Second, "4", false); other == firstClient {
		t.Fatal("managers unexpectedly share HTTP clients")
	}
}
