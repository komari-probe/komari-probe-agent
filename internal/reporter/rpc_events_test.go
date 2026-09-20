package reporter

import (
	"context"
	"testing"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
)

func TestV2AckEventIDsSnapshotAndClear(t *testing.T) {
	r := New(Options{}, collector.New(collector.Options{}), nil)
	r.addV2AckEventID("first")
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
