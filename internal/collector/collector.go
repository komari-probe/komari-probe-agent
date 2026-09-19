package collector

// Options contains the collection-specific portion of the Agent configuration.
// A Collector owns an immutable copy for the duration of one Agent run.
type Options struct {
	IncludeNics         string
	ExcludeNics         string
	IncludeMountpoints  string
	MonthRotate         int
	MemoryIncludeCache  bool
	MemoryReportRawUsed bool
	CustomIPv4          string
	CustomIPv6          string
	GetIPAddrFromNIC    bool
	HostProc            string
	EnableGPU           bool
}

// Collector gathers host metrics using the supplied runtime options.
type Collector struct {
	options Options
}

func New(options Options) *Collector {
	return &Collector{options: options}
}

// ProcessCount returns the number of processes visible to this collector.
func (c *Collector) ProcessCount() int {
	return processCount(c.options.HostProc)
}
