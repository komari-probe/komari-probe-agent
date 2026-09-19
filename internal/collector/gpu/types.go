// Package gpu provides platform-specific GPU discovery and metrics collection.
package gpu

// Device contains the metrics reported for one GPU device.
type Device struct {
	Name        string  `json:"name"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUsed  uint64  `json:"memory_used"`
	Utilization float64 `json:"utilization"`
	Temperature uint64  `json:"temperature"`
}
