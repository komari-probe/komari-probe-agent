package app

import (
	"context"
	"errors"
	"testing"

	"github.com/sonar-probe/sonar-agent/internal/config"
)

func validConfig() config.Config {
	cfg := config.Default()
	cfg.Endpoint = "https://panel.example.test"
	cfg.Token = "token"
	return cfg
}

type fakeRuntime struct {
	runErr   error
	closeErr error
	runCalls int
	closes   int
}

func (r *fakeRuntime) Run(context.Context) error {
	r.runCalls++
	return r.runErr
}

func (r *fakeRuntime) Close() error {
	r.closes++
	return r.closeErr
}

func TestRunRejectsInvalidConfigurationBeforeStartingServices(t *testing.T) {
	err := run(context.Background(), config.Config{})
	if err == nil {
		t.Fatal("run() accepted an invalid configuration")
	}
}

func TestRunRejectsInvalidConfigurationThroughPublicEntryPoint(t *testing.T) {
	if err := Run(config.Config{}); err == nil {
		t.Fatal("Run() accepted an invalid configuration")
	}
}

func TestRunWithFactoryRejectsInvalidConfigurationBeforeBuildingRuntime(t *testing.T) {
	built := false
	err := runWithFactory(context.Background(), config.Config{}, func(context.Context, config.Config) (runtime, error) {
		built = true
		return &fakeRuntime{}, nil
	})
	if err == nil {
		t.Fatal("runWithFactory() accepted an invalid configuration")
	}
	if built {
		t.Fatal("runWithFactory() built a runtime for invalid configuration")
	}
}

func TestRunWithFactoryClosesRuntimeAfterRun(t *testing.T) {
	wantErr := errors.New("reporting failed")
	fake := &fakeRuntime{runErr: wantErr}
	err := runWithFactory(context.Background(), validConfig(), func(context.Context, config.Config) (runtime, error) {
		return fake, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWithFactory() error = %v, want %v", err, wantErr)
	}
	if fake.runCalls != 1 || fake.closes != 1 {
		t.Fatalf("runtime calls = run %d, close %d; want one each", fake.runCalls, fake.closes)
	}
}

func TestRunWithFactoryTreatsCanceledContextAsCleanShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeRuntime{runErr: errors.New("context canceled")}
	err := runWithFactory(ctx, validConfig(), func(context.Context, config.Config) (runtime, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("runWithFactory() error = %v, want nil after cancellation", err)
	}
	if fake.closes != 1 {
		t.Fatalf("runtime closes = %d, want 1", fake.closes)
	}
}

type fakeCollector struct {
	initialized    bool
	closed         bool
	diskListErr    error
	interfaceError error
}

func (c *fakeCollector) InitNetStatic() { c.initialized = true }
func (c *fakeCollector) Close() error {
	c.closed = true
	return nil
}
func (c *fakeCollector) DiskList() ([]string, error) { return []string{"/"}, c.diskListErr }
func (c *fakeCollector) InterfaceList() ([]string, error) {
	return []string{"eth0"}, c.interfaceError
}

type fakeReporter struct {
	runCalls    int
	staticCalls int
}

func (r *fakeReporter) Run(context.Context) error {
	r.runCalls++
	return nil
}
func (r *fakeReporter) RunStaticInfoReporter(ctx context.Context) {
	r.staticCalls++
	<-ctx.Done()
}

func TestAgentRuntimeStartsAndStopsServices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	collector := &fakeCollector{diskListErr: errors.New("disk unavailable")}
	reporter := &fakeReporter{}
	runtime := &agentRuntime{collector: collector, reporter: reporter}
	if err := runtime.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !collector.initialized {
		t.Fatal("Run() did not initialize network traffic collection")
	}
	if reporter.runCalls != 1 || reporter.staticCalls != 1 {
		t.Fatalf("reporter calls = run %d, static %d; want one each", reporter.runCalls, reporter.staticCalls)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !collector.closed {
		t.Fatal("Close() did not close the collector")
	}
}
