package collector

import "github.com/komari-probe/komari-probe-agent/internal/collector/netstatic"

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
}

// Collector gathers host metrics using the supplied runtime options.
type Collector struct {
	options        Options
	networkSpeed   networkSpeedState
	trafficTracker *netstatic.Tracker
}

func New(options Options) *Collector {
	return &Collector{
		options:        options,
		trafficTracker: netstatic.NewTracker(""),
	}
}

// ProcessCount returns the number of processes visible to this collector.
func (c *Collector) ProcessCount() int {
	return processCount(c.options.HostProc)
}
