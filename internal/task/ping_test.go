package task

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestTCPPingRejectsInvalidPort(t *testing.T) {
	latency, err := tcpPing("127.0.0.1:not-a-port", time.Second)
	if err == nil {
		t.Fatal("tcpPing() succeeded with an invalid port")
	}
	if latency != -1 {
		t.Fatalf("tcpPing() latency = %d, want -1 on failure", latency)
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

	withoutScheme := strings.TrimPrefix(successServer.URL, "http://")
	if _, err := httpPing(withoutScheme, time.Second); err != nil {
		t.Fatalf("httpPing() without a scheme error = %v", err)
	}

	failureServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer failureServer.Close()

	if _, err := httpPing(failureServer.URL, time.Second); err == nil {
		t.Fatal("httpPing() succeeded for an HTTP 500 response")
	}
}

func TestHTTPPingTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	latency, err := httpPing(server.URL, 10*time.Millisecond)
	if err == nil {
		t.Fatal("httpPing() succeeded after its timeout")
	}
	if latency != -1 {
		t.Fatalf("httpPing() latency = %d, want -1 on timeout", latency)
	}
}

func TestProbeUsesTCPMeasurement(t *testing.T) {
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

	latency, err := Probe("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if latency < 0 {
		t.Fatalf("Probe() latency = %d, want non-negative", latency)
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("Probe() did not connect to the TCP listener")
	}
}
