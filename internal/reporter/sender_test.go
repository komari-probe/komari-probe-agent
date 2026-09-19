package reporter

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
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
