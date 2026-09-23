package collector

import "github.com/sonar-probe/sonar-agent/internal/collector/gpu"

// GPUDevice is the metrics reported for one GPU device.
type GPUDevice = gpu.Device

// GPUName returns a display name for the host GPUs.
func (c *Collector) GPUName() string {
	return gpu.Name()
}

// GPUModelNames returns the detected GPU model names when detailed monitoring
// is supported by the current platform and driver.
func (c *Collector) GPUModelNames() ([]string, error) {
	return gpu.ModelNames()
}

// GPUDevices returns detailed metrics for each detected GPU device.
func (c *Collector) GPUDevices() ([]GPUDevice, error) {
	return gpu.Devices()
}
