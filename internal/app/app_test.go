package app

import (
	"context"
	"testing"

	"github.com/komari-probe/komari-probe-agent/internal/config"
)

func TestRunRejectsInvalidConfigurationBeforeStartingServices(t *testing.T) {
	err := run(context.Background(), config.Config{})
	if err == nil {
		t.Fatal("run() accepted an invalid configuration")
	}
}
