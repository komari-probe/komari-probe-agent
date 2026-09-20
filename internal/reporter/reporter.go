package reporter

import (
	"sync"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
)

// Options contains the reporting and panel-connectivity configuration for one
// Agent run.
type Options struct {
	Endpoint           string
	Token              string
	Interval           float64
	MaxRetries         int
	ReconnectInterval  int
	InfoReportInterval int
	IgnoreUnsafeCert   bool
	DisableCompression bool
	PreferIPVersion    string
	EnableGPU          bool
}

// Reporter owns report delivery state for one Agent run.
type Reporter struct {
	options     Options
	collector   *collector.Collector
	connections *connectivity.Manager

	v2AckMu       sync.Mutex
	v2AckEventIDs []string
	v2SeenEvents  map[string]time.Time
}

func New(options Options, hostCollector *collector.Collector, connections *connectivity.Manager) *Reporter {
	if connections == nil {
		connections = connectivity.NewManager(connectivity.Options{})
	}
	return &Reporter{
		options:      options,
		collector:    hostCollector,
		connections:  connections,
		v2SeenEvents: make(map[string]time.Time),
	}
}
