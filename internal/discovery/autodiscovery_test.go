package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
)

func discoveryConfig(endpoint string) config.Config {
	return config.Config{Endpoint: endpoint, AutoDiscoveryKey: "discovery-key"}
}

func TestRegisterWithAutoDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/clients/register" {
			t.Fatalf("request = %s %s, want POST /api/clients/register", request.Method, request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer discovery-key" {
			t.Fatalf("Authorization = %q, want discovery key", got)
		}
		var body registrationRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Key != "discovery-key" {
			t.Fatalf("request key = %q", body.Key)
		}
		_, _ = writer.Write([]byte(`{"status":"success","data":{"uuid":"agent-1","token":"token-1"}}`))
	}))
	defer server.Close()

	credentials, err := registerWithAutoDiscovery(context.Background(), discoveryConfig(server.URL), connectivity.NewManager(connectivity.Options{}))
	if err != nil {
		t.Fatalf("registerWithAutoDiscovery() error = %v", err)
	}
	if credentials.UUID != "agent-1" || credentials.Token != "token-1" {
		t.Fatalf("credentials = %#v", credentials)
	}
}

func TestRegisterWithAutoDiscoveryRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte("invalid key"))
	}))
	defer server.Close()

	if _, err := registerWithAutoDiscovery(context.Background(), discoveryConfig(server.URL), connectivity.NewManager(connectivity.Options{})); err == nil {
		t.Fatal("registerWithAutoDiscovery() succeeded for HTTP 403")
	}
}

func TestRegisterWithAutoDiscoveryRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("not-json"))
	}))
	defer server.Close()

	if _, err := registerWithAutoDiscovery(context.Background(), discoveryConfig(server.URL), connectivity.NewManager(connectivity.Options{})); err == nil {
		t.Fatal("registerWithAutoDiscovery() accepted invalid JSON")
	}
}

func TestRegisterWithAutoDiscoveryRejectsFailedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"status":"error","message":"registration rejected"}`))
	}))
	defer server.Close()

	if _, err := registerWithAutoDiscovery(context.Background(), discoveryConfig(server.URL), connectivity.NewManager(connectivity.Options{})); err == nil {
		t.Fatal("registerWithAutoDiscovery() accepted failed registration status")
	}
}

func TestRegisterWithAutoDiscoveryHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := registerWithAutoDiscovery(ctx, discoveryConfig("http://127.0.0.1:1"), connectivity.NewManager(connectivity.Options{})); err == nil {
		t.Fatal("registerWithAutoDiscovery() succeeded with a canceled context")
	}
}
