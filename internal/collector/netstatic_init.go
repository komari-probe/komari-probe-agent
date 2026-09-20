package collector

import (
	"log"

	"github.com/komari-probe/komari-probe-agent/internal/collector/netstatic"
)

// InitNetStatic starts netstatic when monthly traffic resets are enabled and
// synchronizes its network-interface configuration.
func (c *Collector) InitNetStatic() {
	if c.options.MonthRotate == 0 {
		return
	}
	err := netstatic.StartOrContinue()
	if err != nil {
		log.Println("Failed to start netstatic monitoring:", err)
	}
	nicNames, err := c.InterfaceList()
	if err != nil {
		log.Println("Failed to get interface list for netstatic:", err)
	}
	err = netstatic.SetNewConfig(netstatic.NetStaticConfig{
		NICs: nicNames,
	})
	if err != nil {
		log.Println("Failed to set netstatic config:", err)
	}
}
