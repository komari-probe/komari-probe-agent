package reporter

import (
	"context"
	"encoding/json"
	log "github.com/komari-probe/komari-probe-agent/internal/logging"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	v2 "github.com/komari-probe/komari-probe-agent/internal/rpc/v2"
)

const (
	v2SeenEventTTL   = 10 * time.Minute
	v2SeenEventLimit = 4096
)

func (r *Reporter) handleWebSocketMessages(ctx context.Context, conn *connectivity.SafeConn, done chan<- struct{}) {
	defer close(done)
	for {
		_, rawMessage, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			return
		}
		var message v2.Request
		if err := json.Unmarshal(rawMessage, &message); err != nil {
			log.Println("Bad ws message:", err)
			continue
		}
		if message.JSONRPC != v2.Version {
			log.Printf("Bad v2 ws message version %q", message.JSONRPC)
			continue
		}
		r.processV2Event(ctx, conn, message.Method, message.Params, "")
	}
}

func (r *Reporter) processV2ResponseEvents(ctx context.Context, resp *v2.Response) {
	if resp == nil || resp.Result == nil {
		return
	}
	var result v2.EventResult
	if err := v2.BindResult(resp.Result, &result); err != nil {
		log.Println("Failed to bind v2 event result:", err)
		return
	}
	for _, event := range result.Events {
		if r.processV2Event(ctx, nil, event.Method, event.Params, event.ID) {
			r.addV2AckEventID(event.ID)
		}
	}
}

func (r *Reporter) processV2Event(ctx context.Context, conn *connectivity.SafeConn, method string, params any, eventID string) bool {
	if ctx.Err() != nil {
		return false
	}
	switch method {
	case v2.MethodAgentPing:
		var ping struct {
			TaskID uint   `json:"ping_task_id"`
			Type   string `json:"ping_type"`
			Target string `json:"ping_target"`
		}
		if err := v2.BindParams(params, &ping); err != nil {
			log.Printf("bad v2 ping params: %v", err)
			return false
		}
		if ping.TaskID == 0 || ping.Target == "" || !isSupportedPingType(ping.Type) {
			log.Printf("invalid v2 ping task: id=%d type=%q target=%q", ping.TaskID, ping.Type, ping.Target)
			return false
		}
		if !r.markV2EventSeen(eventID) {
			return true
		}
		go r.reportPingTask(ctx, conn, ping.TaskID, ping.Type, ping.Target)
		return true
	case v2.MethodAgentMessage, v2.MethodAgentEvent:
		if !r.markV2EventSeen(eventID) {
			return true
		}
		log.Printf("received v2 %s: %+v", method, params)
		return true
	default:
		log.Printf("unknown v2 event method %s", method)
		return false
	}
}

func isSupportedPingType(pingType string) bool {
	switch pingType {
	case "icmp", "tcp", "http":
		return true
	default:
		return false
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
	for _, existingID := range r.v2AckEventIDs {
		if existingID == id {
			return
		}
	}
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
