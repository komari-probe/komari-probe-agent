package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	v2 "github.com/komari-probe/komari-probe-agent/internal/rpc/v2"
	"github.com/komari-probe/komari-probe-agent/pkg/idna"
)

const (
	v2SeenEventTTL   = 10 * time.Minute
	v2SeenEventLimit = 4096
)

func (r *Reporter) EstablishWebSocketConnection() {
	var conn *connectivity.SafeConn
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	var err error
	interval := math.Max(1, r.options.Interval)

	// Connection recovery must not wait for the (possibly much longer) report
	// interval. Poll the connection frequently and gate reports separately.
	dataTicker := time.NewTicker(time.Second)
	defer dataTicker.Stop()
	reportInterval := time.Duration(interval * float64(time.Second))
	nextReportAt := time.Now()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	var readDone <-chan struct{}

	for {
		select {
		case <-dataTicker.C:
			if conn == nil {
				log.Println("Attempting to connect to WebSocket...")
				retry := 0
				for retry <= r.options.MaxRetries {
					if retry > 0 {
						log.Println("Retrying websocket connection, attempt:", retry)
					}
					websocketEndpoint := r.buildWebSocketEndpoint()
					conn, err = r.connectWebSocket(websocketEndpoint)
					if err == nil {
						log.Println("WebSocket connected using v2 protocol")
						done := make(chan struct{})
						readDone = done
						go r.handleWebSocketMessages(conn, done)
						break
					} else {
						log.Println("Failed to connect to WebSocket:", err)
					}
					retry++
					time.Sleep(time.Duration(r.options.ReconnectInterval) * time.Second)
				}

				if retry > r.options.MaxRetries {
					log.Println("Max retries reached.")
					conn, err = r.runPostFallback(r.buildWebSocketEndpoint(), interval)
					if err != nil {
						log.Println("POST fallback stopped:", err)
						return
					}
					log.Println("WebSocket recovered from POST fallback")
					done := make(chan struct{})
					readDone = done
					go r.handleWebSocketMessages(conn, done)
				}
			}
			if conn == nil || time.Now().Before(nextReportAt) {
				continue
			}
			nextReportAt = time.Now().Add(reportInterval)

			data := buildReportPayload(r.GenerateReport())
			err = r.sendRPCPayload(conn, data)
			if err != nil {
				log.Println("Failed to send WebSocket message:", err)
				conn.Close()
				conn = nil // Mark connection as dead
				readDone = nil
				continue
			}
		case <-heartbeatTicker.C:
			if conn != nil {
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					log.Println("Failed to send heartbeat:", err)
					conn.Close()
					conn = nil // Mark connection as dead
					readDone = nil
				}
			}
		case <-readDone:
			log.Println("WebSocket disconnected")
			if conn != nil {
				conn.Close()
				conn = nil
			}
			readDone = nil
		}
	}
}

func (r *Reporter) buildWebSocketEndpoint() string {
	websocketEndpoint := strings.TrimSuffix(r.options.Endpoint, "/") + "/api/clients/v2/rpc?token=" + r.options.Token
	websocketEndpoint = "ws" + strings.TrimPrefix(websocketEndpoint, "http")
	if convertedEndpoint, err := idna.ConvertIDNToASCII(websocketEndpoint); err == nil {
		return convertedEndpoint
	} else {
		log.Printf("Warning: Failed to convert WebSocket IDN to ASCII: %v", err)
	}
	return websocketEndpoint
}

func (r *Reporter) runPostFallback(websocketEndpoint string, interval float64) (*connectivity.SafeConn, error) {
	log.Println("Entering v2 POST fallback mode")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.runV2PullLoop(ctx)

	reportTicker := time.NewTicker(time.Duration(interval * float64(time.Second)))
	defer reportTicker.Stop()
	reconnectTicker := time.NewTicker(time.Duration(r.options.ReconnectInterval) * time.Second)
	defer reconnectTicker.Stop()

	for {
		select {
		case <-reportTicker.C:
			reportID := fmt.Sprintf("report-%d", time.Now().UnixNano())
			ackIDs := r.snapshotV2AckEventIDs()
			resp, err := r.postV2Request(buildReportRequest(reportID, r.GenerateReport(), ackIDs))
			if err != nil {
				log.Println("Failed to POST v2 report:", err)
				continue
			}
			r.clearV2AckEventIDs(ackIDs)
			r.processV2ResponseEvents(resp)
		case <-reconnectTicker.C:
			conn, err := r.connectWebSocket(websocketEndpoint)
			if err == nil {
				return conn, nil
			}
			log.Println("POST fallback WebSocket recovery failed:", err)
		}
	}
}

func (r *Reporter) runV2PullLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		pullID := fmt.Sprintf("pull-%d", time.Now().UnixNano())
		ackIDs := r.snapshotV2AckEventIDs()
		payload := v2.NewRequest(pullID, v2.MethodAgentPull, map[string]any{
			"capabilities":  []string{"ping", "message", "event"},
			"ack_event_ids": ackIDs,
		})
		resp, err := r.postV2RequestContext(ctx, payload)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Println("Failed to POST v2 pull:", err)
			timer := time.NewTimer(time.Duration(r.options.ReconnectInterval) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		r.clearV2AckEventIDs(ackIDs)
		r.processV2ResponseEvents(resp)
	}
}

func (r *Reporter) postV2Request(payload []byte) (*v2.Response, error) {
	return r.postV2RequestContext(context.Background(), payload)
}

func (r *Reporter) postV2RequestContext(ctx context.Context, payload []byte) (*v2.Response, error) {
	bytesBody, err := r.postRPCPayload(ctx, payload, 35*time.Second)
	if err != nil {
		return nil, err
	}
	rpcResp, err := v2.ParseResponse(bytesBody)
	if err != nil {
		return nil, err
	}
	return rpcResp, nil
}

func (r *Reporter) processV2ResponseEvents(resp *v2.Response) {
	if resp == nil || resp.Result == nil {
		return
	}
	var result v2.EventResult
	if err := v2.BindResult(resp.Result, &result); err != nil {
		log.Println("Failed to bind v2 event result:", err)
		return
	}
	for _, event := range result.Events {
		if r.processV2Event(nil, event.Method, event.Params, event.ID) {
			r.addV2AckEventID(event.ID)
		}
	}
}

func (r *Reporter) snapshotV2AckEventIDs() []string {
	r.v2AckMu.Lock()
	defer r.v2AckMu.Unlock()
	return append([]string{}, r.v2AckEventIDs...)
}

func (r *Reporter) clearV2AckEventIDs(sent []string) {
	if len(sent) == 0 {
		return
	}
	sentSet := make(map[string]struct{}, len(sent))
	for _, id := range sent {
		sentSet[id] = struct{}{}
	}
	r.v2AckMu.Lock()
	defer r.v2AckMu.Unlock()
	remaining := r.v2AckEventIDs[:0]
	for _, id := range r.v2AckEventIDs {
		if _, ok := sentSet[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	r.v2AckEventIDs = remaining
}

func (r *Reporter) addV2AckEventID(id string) {
	if id == "" {
		return
	}
	r.v2AckMu.Lock()
	defer r.v2AckMu.Unlock()
	r.v2AckEventIDs = append(r.v2AckEventIDs, id)
}

func (r *Reporter) markV2EventSeen(id string) bool {
	if id == "" {
		return true
	}
	r.v2AckMu.Lock()
	defer r.v2AckMu.Unlock()
	now := time.Now()
	for eventID, seenAt := range r.v2SeenEvents {
		if now.Sub(seenAt) > v2SeenEventTTL {
			delete(r.v2SeenEvents, eventID)
		}
	}
	if _, ok := r.v2SeenEvents[id]; ok {
		return false
	}
	if len(r.v2SeenEvents) >= v2SeenEventLimit {
		var oldestID string
		var oldest time.Time
		for eventID, seenAt := range r.v2SeenEvents {
			if oldestID == "" || seenAt.Before(oldest) {
				oldestID, oldest = eventID, seenAt
			}
		}
		if oldestID != "" {
			delete(r.v2SeenEvents, oldestID)
		}
	}
	r.v2SeenEvents[id] = now
	return true
}

func (r *Reporter) connectWebSocket(websocketEndpoint string) (*connectivity.SafeConn, error) {
	dialer := connectivity.NewWebSocketDialer(connectivity.WebSocketDialerOptions{
		HandshakeTimeout:  15 * time.Second,
		DialTimeout:       15 * time.Second,
		PreferIPVersion:   r.options.PreferIPVersion,
		IgnoreUnsafeCert:  r.options.IgnoreUnsafeCert,
		EnableCompression: !r.options.DisableCompression,
	})

	conn, resp, err := dialer.Dial(websocketEndpoint, nil)
	if err != nil {
		if resp != nil && resp.StatusCode != 101 {
			return nil, &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
		}
		return nil, err
	}

	return connectivity.NewSafeConn(conn), nil
}

func (r *Reporter) handleWebSocketMessages(conn *connectivity.SafeConn, done chan<- struct{}) {
	defer close(done)
	for {
		_, message_raw, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			return
		}
		var message v2.Request
		err = json.Unmarshal(message_raw, &message)
		if err != nil {
			log.Println("Bad ws message:", err)
			continue
		}
		if message.JSONRPC != v2.Version {
			log.Printf("Bad v2 ws message version %q", message.JSONRPC)
			continue
		}
		r.processV2Event(conn, message.Method, message.Params, "")
	}
}

func (r *Reporter) processV2Event(conn *connectivity.SafeConn, method string, params any, eventID string) bool {
	if !r.markV2EventSeen(eventID) {
		return true
	}
	switch method {
	case v2.MethodAgentPing:
		var p struct {
			TaskID uint   `json:"ping_task_id"`
			Type   string `json:"ping_type"`
			Target string `json:"ping_target"`
		}
		if err := v2.BindParams(params, &p); err == nil {
			go r.reportPingTask(conn, p.TaskID, p.Type, p.Target)
			return true
		} else {
			log.Printf("bad v2 ping params: %v", err)
		}
	case v2.MethodAgentMessage, v2.MethodAgentEvent:
		log.Printf("received v2 %s: %+v", method, params)
		return true
	default:
		log.Printf("unknown v2 event method %s", method)
	}
	return false
}
