package reporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
)

func newConnectionTestReporter(endpoint string) *Reporter {
	return New(Options{Endpoint: endpoint, Token: "token", Interval: 1, ReconnectInterval: 1}, collector.New(collector.Options{}), connectivity.NewManager(connectivity.Options{}))
}

func TestBuildWebSocketEndpoint(t *testing.T) {
	for _, test := range []struct{ endpoint, want string }{
		{"http://panel.example", "ws://panel.example/api/clients/v2/rpc?token=token"},
		{"https://panel.example/", "wss://panel.example/api/clients/v2/rpc?token=token"},
	} {
		if got := newConnectionTestReporter(test.endpoint).buildWebSocketEndpoint(); got != test.want {
			t.Errorf("buildWebSocketEndpoint() = %q, want %q", got, test.want)
		}
	}
}

func TestConnectWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		_, _, _ = connection.ReadMessage()
	}))
	defer server.Close()

	reporter := newConnectionTestReporter(server.URL)
	connection, err := reporter.connectWebSocket(context.Background(), reporter.buildWebSocketEndpoint())
	if err != nil {
		t.Fatalf("connectWebSocket() error = %v", err)
	}
	defer connection.Close()
	if err := connection.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
}

func TestConnectWebSocketReturnsHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	reporter := newConnectionTestReporter(server.URL)
	_, err := reporter.connectWebSocket(context.Background(), reporter.buildWebSocketEndpoint())
	var statusError *HTTPStatusError
	if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("connectWebSocket() error = %v, want HTTP 503 error", err)
	}
}

func TestPostRPCPayloadCompressesRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/clients/v2/rpc" || request.URL.Query().Get("token") != "token" {
			t.Fatalf("request URL = %s", request.URL)
		}
		if request.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("Content-Encoding = %q, want gzip", request.Header.Get("Content-Encoding"))
		}
		reader, err := gzip.NewReader(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, []byte(`{"method":"agent.report"}`)) {
			t.Fatalf("request body = %s", body)
		}
		_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","result":{}}`))
	}))
	defer server.Close()

	body, err := newConnectionTestReporter(server.URL).postRPCPayload(context.Background(), []byte(`{"method":"agent.report"}`), time.Second)
	if err != nil {
		t.Fatalf("postRPCPayload() error = %v", err)
	}
	if len(body) == 0 {
		t.Fatal("postRPCPayload() returned an empty response")
	}
}

func TestPostRPCPayloadReturnsHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("upstream unavailable"))
	}))
	defer server.Close()

	_, err := newConnectionTestReporter(server.URL).postRPCPayload(context.Background(), []byte(`{}`), time.Second)
	var statusError *HTTPStatusError
	if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusBadGateway || statusError.Body != "upstream unavailable" {
		t.Fatalf("postRPCPayload() error = %#v", err)
	}
}

func TestPostAndValidateRPCAcceptsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	if err := newConnectionTestReporter(server.URL).postAndValidateRPC(context.Background(), []byte(`{}`), time.Second); err != nil {
		t.Fatalf("postAndValidateRPC() error = %v", err)
	}
}

func TestPostAndValidateRPCRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("not-json"))
	}))
	defer server.Close()

	if err := newConnectionTestReporter(server.URL).postAndValidateRPC(context.Background(), []byte(`{}`), time.Second); err == nil {
		t.Fatal("postAndValidateRPC() accepted an invalid RPC response")
	}
}

func TestSendRPCPayloadUsesWebSocket(t *testing.T) {
	received := make(chan []byte, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		_, payload, err := connection.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		received <- payload
	}))
	defer server.Close()

	reporter := newConnectionTestReporter(server.URL)
	connection, err := reporter.connectWebSocket(context.Background(), reporter.buildWebSocketEndpoint())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := reporter.sendRPCPayload(context.Background(), connection, []byte(`{"method":"agent.report"}`)); err != nil {
		t.Fatalf("sendRPCPayload() error = %v", err)
	}
	select {
	case payload := <-received:
		if !bytes.Equal(payload, []byte(`{"method":"agent.report"}`)) {
			t.Fatalf("payload = %s", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("WebSocket server did not receive an RPC payload")
	}
}
