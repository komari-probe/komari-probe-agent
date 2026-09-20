package reporter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	v2 "github.com/komari-probe/komari-probe-agent/internal/rpc/v2"
)

// runPostFallback exchanges v2 reports over HTTP until WebSocket connectivity returns.
func (r *Reporter) runPostFallback(ctx context.Context, endpoint string, interval float64) (*connectivity.SafeConn, error) {
	log.Println("Entering v2 POST fallback mode")
	pullCtx, cancelPull := context.WithCancel(ctx)
	pullDone := make(chan struct{})
	go func() { defer close(pullDone); r.runV2PullLoop(pullCtx) }()
	defer func() { cancelPull(); <-pullDone }()

	reportTicker := time.NewTicker(time.Duration(interval * float64(time.Second)))
	defer reportTicker.Stop()
	reconnectTicker := time.NewTicker(time.Duration(r.options.ReconnectInterval) * time.Second)
	defer reconnectTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-reportTicker.C:
			r.sendFallbackReport(ctx)
		case <-reconnectTicker.C:
			conn, err := r.connectWebSocket(ctx, endpoint)
			if err == nil {
				return conn, nil
			}
			log.Println("POST fallback WebSocket recovery failed:", err)
		}
	}
}

func (r *Reporter) sendFallbackReport(ctx context.Context) {
	ackIDs := r.snapshotV2AckEventIDs()
	report, err := r.GenerateReport()
	if err != nil {
		log.Println("Failed to build performance report:", err)
		return
	}
	payload, err := buildReportRequest(fmt.Sprintf("report-%d", time.Now().UnixNano()), report, ackIDs)
	if err != nil {
		log.Println("Failed to build POST report payload:", err)
		return
	}
	resp, err := r.postV2Request(ctx, payload)
	if err != nil {
		log.Println("Failed to POST v2 report:", err)
		return
	}
	r.clearV2AckEventIDs(ackIDs)
	r.processV2ResponseEvents(ctx, resp)
}

func (r *Reporter) runV2PullLoop(ctx context.Context) {
	for ctx.Err() == nil {
		ackIDs := r.snapshotV2AckEventIDs()
		payload, err := v2.NewRequest(fmt.Sprintf("pull-%d", time.Now().UnixNano()), v2.MethodAgentPull, map[string]any{
			"capabilities": []string{"ping", "message", "event"}, "ack_event_ids": ackIDs,
		})
		if err != nil {
			log.Println("Failed to build v2 pull payload:", err)
			return
		}
		resp, err := r.postV2RequestContext(ctx, payload)
		if err == nil {
			r.clearV2AckEventIDs(ackIDs)
			r.processV2ResponseEvents(ctx, resp)
			continue
		}
		if ctx.Err() != nil {
			return
		}
		log.Println("Failed to POST v2 pull:", err)
		if !waitForContext(ctx, time.Duration(r.options.ReconnectInterval)*time.Second) {
			return
		}
	}
}

func (r *Reporter) postV2Request(ctx context.Context, payload []byte) (*v2.Response, error) {
	return r.postV2RequestContext(ctx, payload)
}

func (r *Reporter) postV2RequestContext(ctx context.Context, payload []byte) (*v2.Response, error) {
	body, err := r.postRPCPayload(ctx, payload, 35*time.Second)
	if err != nil {
		return nil, err
	}
	return v2.ParseResponse(body)
}
