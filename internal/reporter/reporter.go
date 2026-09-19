package reporter

import (
	"sync"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
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
	options   Options
	collector *collector.Collector

	v2AckMu       sync.Mutex
	v2AckEventIDs []string
	v2SeenEvents  map[string]time.Time
}

func New(options Options, hostCollector *collector.Collector) *Reporter {
	return &Reporter{
		options:      options,
		collector:    hostCollector,
		v2SeenEvents: make(map[string]time.Time),
	}
}
