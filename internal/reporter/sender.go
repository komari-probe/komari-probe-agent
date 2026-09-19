package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/internal/protocol/transport"
	v2 "github.com/komari-probe/komari-probe-agent/internal/protocol/v2"
)

// sendRPC serializes a v2 RPC value and delivers it over the active WebSocket
// connection, falling back to HTTP when no connection is available.
func sendRPC(conn *connectivity.SafeConn, payload interface{}) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return sendRPCPayload(conn, encoded)
}

func sendRPCPayload(conn *connectivity.SafeConn, payload []byte) error {
	if conn != nil {
		return conn.WriteMessage(websocket.TextMessage, payload)
	}
	return postAndValidateRPC(context.Background(), payload, 30*time.Second)
}

func postAndValidateRPC(ctx context.Context, payload []byte, timeout time.Duration) error {
	response, err := postRPCPayload(ctx, payload, timeout)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(response)) == 0 {
		return nil
	}
	_, err = v2.ParseResponse(response)
	return err
}

// postRPCPayload performs the shared authenticated, compressed HTTP fallback
// request and returns its raw response body for callers that need to process
// v2 events.
func postRPCPayload(ctx context.Context, payload []byte, timeout time.Duration) ([]byte, error) {
	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/v2/rpc?token=" + flags.Token
	body := payload
	compressed := false
	if !flags.DisableCompression {
		if gz, err := transport.GzipBytes(payload); err == nil {
			body = gz
			compressed = true
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := connectivity.GetHTTPClientWithPreference(timeout, flags.PreferIPVersion, flags.IgnoreUnsafeCert)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &v2.HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(response)}
	}
	return response, nil
}
