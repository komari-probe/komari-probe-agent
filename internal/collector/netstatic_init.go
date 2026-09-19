package collector

import (
	"log"

	"github.com/komari-probe/komari-probe-agent/internal/collector/netstatic"
)

// InitNetstatic 在启用了月度重置统计时，启动 netstatic 并同步网卡配置
func (c *Collector) InitNetstatic() {
	if c.options.MonthRotate == 0 {
		return
	}
	err := netstatic.StartOrContinue()
	if err != nil {
		log.Println("Failed to start netstatic monitoring:", err)
	}
	nics, err := c.InterfaceList()
	if err != nil {
		log.Println("Failed to get interface list for netstatic:", err)
	}
	err = netstatic.SetNewConfig(netstatic.NetStaticConfig{
		NICs: nics,
	})
	if err != nil {
		log.Println("Failed to set netstatic config:", err)
	}
}
