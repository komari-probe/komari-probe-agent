package reporter

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"testing"
	"time"

	v2 "github.com/komari-probe/komari-probe-agent/internal/rpc/v2"
)

func TestGzipPayloadRoundTrip(t *testing.T) {
	want := []byte(`{"jsonrpc":"2.0","method":"agent.report"}`)
	compressed, err := gzipPayload(want)
	if err != nil {
		t.Fatalf("gzipPayload() error = %v", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("decompressed payload = %q, want %q", got, want)
	}
}

func TestBuildPingResultPayload(t *testing.T) {
	finishedAt := time.Date(2026, time.September, 20, 12, 34, 56, 0, time.UTC)
	payload := buildPingResultPayload(42, "tcp", 15, finishedAt)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var message v2.Request
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if message.JSONRPC != v2.Version || message.Method != v2.MethodAgentPingResult {
		t.Fatalf("message = %+v, want a ping-result RPC request", message)
	}
	var params struct {
		TaskID     uint   `json:"task_id"`
		PingType   string `json:"ping_type"`
		Value      int    `json:"value"`
		FinishedAt string `json:"finished_at"`
	}
	if err := v2.BindParams(message.Params, &params); err != nil {
		t.Fatalf("BindParams() error = %v", err)
	}
	if params.TaskID != 42 || params.PingType != "tcp" || params.Value != 15 || params.FinishedAt != finishedAt.Format(time.RFC3339Nano) {
		t.Fatalf("ping-result params = %+v", params)
	}
}
