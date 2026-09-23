package collector

import (
	"github.com/sonar-probe/sonar-agent/internal/collector/netstatic"
	"github.com/sonar-probe/sonar-agent/internal/connectivity"
)

// Options contains the collection-specific portion of the Agent configuration.
// A Collector owns an immutable copy for the duration of one Agent run.
type Options struct {
	IncludeNICs         string
	ExcludeNICs         string
	IncludeMountpoints  string
	MonthRotate         int
	MemoryIncludeCache  bool
	MemoryReportRawUsed bool
	CustomIPv4          string
	CustomIPv6          string
	GetIPAddressFromNIC bool
	HostProc            string
	Connectivity        *connectivity.Manager
	IgnoreUnsafeCert    bool
}

// Collector gathers host metrics using the supplied runtime options.
type Collector struct {
	options        Options
	connections    *connectivity.Manager
	networkSpeed   networkSpeedState
	trafficTracker *netstatic.Tracker
}

func New(options Options) *Collector {
	connections := options.Connectivity
	if connections == nil {
		connections = connectivity.NewManager(connectivity.Options{})
	}
	return &Collector{
		options:        options,
		connections:    connections,
		trafficTracker: netstatic.NewTracker(""),
	}
}

// ProcessCount returns the number of processes visible to this collector.
func (c *Collector) ProcessCount() int {
	return processCount(c.options.HostProc)
}
