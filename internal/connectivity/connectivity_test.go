package connectivity

import (
	"reflect"
	"testing"
	"time"
)

func TestNormalizeDNSServer(t *testing.T) {
	tests := map[string]string{
		"1.1.1.1":                   "1.1.1.1:53",
		"1.1.1.1:5353":              "1.1.1.1:5353",
		"2001:4860:4860::8888":      "[2001:4860:4860::8888]:53",
		"[2001:4860:4860::8888]:53": "[2001:4860:4860::8888]:53",
	}
	for input, want := range tests {
		if got := normalizeDNSServer(input); got != want {
			t.Errorf("normalizeDNSServer(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSortIPsByPreference(t *testing.T) {
	manager := NewManager(Options{})
	ips := []string{"2001:db8::1", "192.0.2.1", "2001:db8::2", "192.0.2.2"}
	manager.sortIPsByPreference(ips, "4")
	want := []string{"192.0.2.1", "192.0.2.2", "2001:db8::1", "2001:db8::2"}
	if !reflect.DeepEqual(ips, want) {
		t.Errorf("sortIPsByPreference() = %v, want %v", ips, want)
	}
}

func TestNewWebSocketDialer(t *testing.T) {
	manager := NewManager(Options{})
	dialer := manager.NewWebSocketDialer(WebSocketDialerOptions{
		HandshakeTimeout:  7 * time.Second,
		DialTimeout:       9 * time.Second,
		PreferIPVersion:   "4",
		IgnoreUnsafeCert:  true,
		EnableCompression: true,
	})

	if dialer.HandshakeTimeout != 7*time.Second {
		t.Errorf("HandshakeTimeout = %v, want 7s", dialer.HandshakeTimeout)
	}
	if dialer.NetDialContext == nil {
		t.Error("NetDialContext is nil")
	}
	if !dialer.EnableCompression {
		t.Error("EnableCompression = false, want true")
	}
	if dialer.TLSClientConfig == nil || !dialer.TLSClientConfig.InsecureSkipVerify {
		t.Error("TLSClientConfig does not preserve IgnoreUnsafeCert")
	}
}
