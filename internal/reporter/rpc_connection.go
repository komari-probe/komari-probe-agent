package reporter

import (
	"context"
	log "github.com/komari-probe/komari-probe-agent/internal/logging"
	"math"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/pkg/idna"
)

// Run sends the initial basic information and maintains the reporting connection.
func (r *Reporter) Run(ctx context.Context) error {
	r.UpdateBasicInfo(ctx)
	return r.runWebSocketConnection(ctx)
}

// runWebSocketConnection owns the active WebSocket session and its recovery.
func (r *Reporter) runWebSocketConnection(ctx context.Context) error {
	var conn *connectivity.SafeConn
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	interval := math.Max(1, r.options.Interval)
	dataTicker := time.NewTicker(time.Second)
	defer dataTicker.Stop()
	reportInterval := time.Duration(interval * float64(time.Second))
	nextReportAt := time.Now()
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	var readDone <-chan struct{}
	for {
		select {
		case <-ctx.Done():
			if conn != nil {
				_ = conn.Close()
			}
			if readDone != nil {
				<-readDone
			}
			return nil
		case <-dataTicker.C:
			if conn == nil {
				var err error
				conn, err = r.connectWithFallback(ctx, interval)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
				done := make(chan struct{})
				readDone = done
				go r.handleWebSocketMessages(ctx, conn, done)
			}
			if time.Now().Before(nextReportAt) {
				continue
			}
			nextReportAt = time.Now().Add(reportInterval)
			if !r.sendPeriodicReport(ctx, conn) {
				conn = nil
				readDone = nil
			}
		case <-heartbeatTicker.C:
			if conn != nil {
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Println("Failed to send heartbeat:", err)
					_ = conn.Close()
					conn = nil
					readDone = nil
				}
			}
		case <-readDone:
			log.Println("WebSocket disconnected")
			if conn != nil {
				_ = conn.Close()
				conn = nil
			}
			readDone = nil
		}
	}
}

func (r *Reporter) connectWithFallback(ctx context.Context, interval float64) (*connectivity.SafeConn, error) {
	log.Debugln("Attempting to connect to WebSocket...")
	endpoint := r.buildWebSocketEndpoint()
	for retry := 0; retry <= r.options.MaxRetries; retry++ {
		if retry > 0 {
			log.Debugln("Retrying websocket connection, attempt:", retry)
		}
		conn, err := r.connectWebSocket(ctx, endpoint)
		if err == nil {
			log.Debugln("WebSocket connected using v2 protocol")
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Debugln("Failed to connect to WebSocket:", err)
		if !waitForContext(ctx, time.Duration(r.options.ReconnectInterval)*time.Second) {
			return nil, ctx.Err()
		}
	}

	log.Println("Max retries reached.")
	conn, err := r.runPostFallback(ctx, endpoint, interval)
	if err != nil {
		return nil, err
	}
	log.Println("WebSocket recovered from POST fallback")
	return conn, nil
}

func (r *Reporter) sendPeriodicReport(ctx context.Context, conn *connectivity.SafeConn) bool {
	report, err := r.GenerateReport()
	if err != nil {
		log.Println("Failed to build performance report:", err)
		return true
	}
	payload, err := buildReportPayload(report)
	if err != nil {
		log.Println("Failed to build WebSocket report payload:", err)
		return true
	}
	if err := r.sendRPCPayload(ctx, conn, payload); err != nil {
		log.Println("Failed to send WebSocket message:", err)
		_ = conn.Close()
		return false
	}
	return true
}

func (r *Reporter) buildWebSocketEndpoint() string {
	endpoint := strings.TrimSuffix(r.options.Endpoint, "/") + "/api/clients/v2/rpc?token=" + r.options.Token
	endpoint = "ws" + strings.TrimPrefix(endpoint, "http")
	if converted, err := idna.ConvertIDNToASCII(endpoint); err == nil {
		return converted
	} else {
		log.Printf("Warning: Failed to convert WebSocket IDN to ASCII: %v", err)
	}
	return endpoint
}

func (r *Reporter) connectWebSocket(ctx context.Context, endpoint string) (*connectivity.SafeConn, error) {
	dialer := r.connections.NewWebSocketDialer(connectivity.WebSocketDialerOptions{
		HandshakeTimeout: 15 * time.Second, DialTimeout: 15 * time.Second,
		PreferIPVersion: r.options.PreferIPVersion, IgnoreUnsafeCert: r.options.IgnoreUnsafeCert,
		EnableCompression: !r.options.DisableCompression,
	})
	conn, resp, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		if resp != nil && resp.StatusCode != 101 {
			return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
		}
		return nil, err
	}
	return connectivity.NewSafeConn(conn), nil
}

func waitForContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
