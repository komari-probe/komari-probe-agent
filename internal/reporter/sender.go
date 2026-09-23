package reporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sonar-probe/sonar-agent/internal/connectivity"
	v2 "github.com/sonar-probe/sonar-agent/internal/rpc/v2"
)

// HTTPStatusError describes an unsuccessful HTTP fallback response.
type HTTPStatusError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body != "" {
		return fmt.Sprintf("status code %d: %s", e.StatusCode, e.Body)
	}
	if e.Status != "" {
		return e.Status
	}
	return fmt.Sprintf("status code: %d", e.StatusCode)
}

// sendRPC serializes a v2 RPC value and delivers it over the active WebSocket
// connection, falling back to HTTP when no connection is available.
func (r *Reporter) sendRPC(ctx context.Context, conn *connectivity.SafeConn, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.sendRPCPayload(ctx, conn, encoded)
}

func (r *Reporter) sendRPCPayload(ctx context.Context, conn *connectivity.SafeConn, payload []byte) error {
	if conn != nil {
		return conn.WriteMessage(websocket.TextMessage, payload)
	}
	return r.postAndValidateRPC(ctx, payload, 30*time.Second)
}

func (r *Reporter) postAndValidateRPC(ctx context.Context, payload []byte, timeout time.Duration) error {
	response, err := r.postRPCPayload(ctx, payload, timeout)
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
func (r *Reporter) postRPCPayload(ctx context.Context, payload []byte, timeout time.Duration) ([]byte, error) {
	endpoint := strings.TrimSuffix(r.options.Endpoint, "/") + "/api/clients/v2/rpc?token=" + r.options.Token
	body := payload
	compressed := false
	if !r.options.DisableCompression {
		gz, err := gzipPayload(payload)
		if err != nil {
			return nil, fmt.Errorf("compress RPC payload: %w", err)
		}
		body = gz
		compressed = true
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := r.connections.NewHTTPClientWithPreference(timeout, r.options.PreferIPVersion, r.options.IgnoreUnsafeCert)
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
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(response)}
	}
	return response, nil
}

func gzipPayload(payload []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
