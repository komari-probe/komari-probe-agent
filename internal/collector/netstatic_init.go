package collector

import (
	log "github.com/komari-probe/komari-probe-agent/internal/logging"
)

// InitNetStatic starts netstatic when monthly traffic resets are enabled and
// synchronizes its network-interface configuration.
func (c *Collector) InitNetStatic() {
	if c.options.MonthRotate == 0 {
		return
	}
	err := c.trafficTracker.Start()
	if err != nil {
		log.Println("Failed to start netstatic monitoring:", err)
	}
	nicNames, err := c.InterfaceList()
	if err != nil {
		log.Println("Failed to get interface list for netstatic:", err)
	}
	err = c.trafficTracker.ConfigureNICs(nicNames)
	if err != nil {
		log.Println("Failed to set netstatic config:", err)
	}
}

// Close flushes and stops the traffic-history collector when it was enabled.
func (c *Collector) Close() error {
	if c.options.MonthRotate == 0 {
		return nil
	}
	return c.trafficTracker.Close()
}
