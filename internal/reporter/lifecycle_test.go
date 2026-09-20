package reporter

import (
	"context"
	"testing"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
)

func TestRunStaticInfoReporterStopsWithContext(t *testing.T) {
	reporter := New(Options{InfoReportInterval: 1}, collector.New(collector.Options{}), nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		reporter.RunStaticInfoReporter(ctx)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("static-info reporter did not stop after context cancellation")
	}
}

func TestWaitForContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForContext(ctx, time.Second) {
		t.Fatal("waitForContext() returned true for a canceled context")
	}
	if !waitForContext(context.Background(), time.Millisecond) {
		t.Fatal("waitForContext() returned false after its timer elapsed")
	}
}
