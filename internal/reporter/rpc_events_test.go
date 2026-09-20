package reporter

import (
	"context"
	"testing"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	v2 "github.com/komari-probe/komari-probe-agent/internal/rpc/v2"
)

func TestV2AckEventIDsSnapshotAndClear(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	r.addV2AckEventID("first")
	r.addV2AckEventID("second")
	r.addV2AckEventID("second")
	r.addV2AckEventID("")

	snapshot := r.snapshotV2AckEventIDs()
	if len(snapshot) != 2 || snapshot[0] != "first" || snapshot[1] != "second" {
		t.Fatalf("snapshotV2AckEventIDs() = %#v, want [first second]", snapshot)
	}
	snapshot[0] = "mutated"
	if got := r.snapshotV2AckEventIDs()[0]; got != "first" {
		t.Fatalf("snapshot mutation changed reporter state to %q", got)
	}

	r.clearV2AckEventIDs([]string{"first"})
	if got := r.snapshotV2AckEventIDs(); len(got) != 1 || got[0] != "second" {
		t.Fatalf("remaining ACK IDs = %#v, want [second]", got)
	}
}

func TestProcessV2ResponseEventsAcknowledgesValidEventsOnce(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	response := &v2.Response{Result: map[string]any{
		"events": []any{map[string]any{
			"id": "message-1", "method": v2.MethodAgentMessage, "params": map[string]any{"text": "hello"},
		}},
	}}
	r.processV2ResponseEvents(context.Background(), response)
	r.processV2ResponseEvents(context.Background(), response)

	if got := r.snapshotV2AckEventIDs(); len(got) != 1 || got[0] != "message-1" {
		t.Fatalf("ACK IDs = %#v, want [message-1]", got)
	}
}

func TestProcessV2ResponseEventsRejectsInvalidPingTask(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	response := &v2.Response{Result: map[string]any{
		"events": []any{map[string]any{
			"id": "ping-1", "method": v2.MethodAgentPing, "params": map[string]any{
				"ping_task_id": 0, "ping_type": "tcp", "ping_target": "127.0.0.1:80",
			},
		}},
	}}
	r.processV2ResponseEvents(context.Background(), response)
	if got := r.snapshotV2AckEventIDs(); len(got) != 0 {
		t.Fatalf("invalid ping task ACK IDs = %#v, want none", got)
	}
}

func TestProcessV2ResponseEventsIgnoresMalformedResult(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	r.processV2ResponseEvents(context.Background(), &v2.Response{Result: "not an event result"})
	if got := r.snapshotV2AckEventIDs(); len(got) != 0 {
		t.Fatalf("malformed result ACK IDs = %#v, want none", got)
	}
}

func TestMarkV2EventSeenDeduplicatesEvents(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	if !r.markV2EventSeen("event-1") {
		t.Fatal("first event should be accepted")
	}
	if r.markV2EventSeen("event-1") {
		t.Fatal("duplicate event should be rejected")
	}
	if !r.markV2EventSeen("") {
		t.Fatal("events without an ID should remain processable")
	}
}

func TestProcessV2EventStopsForCanceledContext(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r.processV2Event(ctx, nil, "agent.message", map[string]any{}, "event-1") {
		t.Fatal("event was processed after context cancellation")
	}
}
