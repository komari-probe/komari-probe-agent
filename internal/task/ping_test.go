package task

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeRejectsUnsupportedType(t *testing.T) {
	result, err := Probe("unsupported", "example.com")
	if err == nil {
		t.Fatal("Probe() succeeded for an unsupported type")
	}
	if result != -1 {
		t.Errorf("Probe() result = %d, want -1", result)
	}
}

func TestResolveIP(t *testing.T) {
	if got, err := resolveIP("127.0.0.1"); err != nil || got != "127.0.0.1" {
		t.Fatalf("resolveIP() = %q, %v; want 127.0.0.1, nil", got, err)
	}
	if _, err := resolveIP("invalid host name"); err == nil {
		t.Fatal("resolveIP() succeeded for an invalid host name")
	}
}

func TestTCPPingLocalListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
			close(accepted)
		}
	}()

	latency, err := tcpPing(listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("tcpPing() error = %v", err)
	}
	if latency < 0 {
		t.Errorf("tcpPing() latency = %d, want non-negative", latency)
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("local TCP listener did not receive a connection")
	}
}

func TestHTTPPing(t *testing.T) {
	successServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer successServer.Close()

	latency, err := httpPing(successServer.URL, time.Second)
	if err != nil {
		t.Fatalf("httpPing() success error = %v", err)
	}
	if latency < 0 {
		t.Errorf("httpPing() latency = %d, want non-negative", latency)
	}

	failureServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer failureServer.Close()

	if _, err := httpPing(failureServer.URL, time.Second); err == nil {
		t.Fatal("httpPing() succeeded for an HTTP 500 response")
	}
}
