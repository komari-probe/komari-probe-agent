package collector

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sonar-probe/sonar-agent/internal/collector/netstatic"
	"github.com/shirou/gopsutil/v4/net"
)

func (c *Collector) ConnectionsCount() (tcpCount, udpCount int, err error) {
	if runtime.GOOS == "linux" {
		return connectionsCountWithProcFallback(c.procRoot(), gopsutilConnectionsCount)
	}

	return gopsutilConnectionsCount()
}

func connectionsCountWithProcFallback(root string, fallback func() (int, int, error)) (tcpCount, udpCount int, err error) {
	var procErr error
	tcpCount, udpCount, procErr = procNetConnectionsCount(root)
	if procErr == nil {
		return tcpCount, udpCount, nil
	}

	tcpCount, udpCount, err = fallback()
	if err != nil && procErr != nil {
		return 0, 0, fmt.Errorf("proc net fast path failed: %w; gopsutil fallback failed: %w", procErr, err)
	}
	return tcpCount, udpCount, err
}

func gopsutilConnectionsCount() (tcpCount, udpCount int, err error) {
	tcps, err := net.Connections("tcp")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get TCP connections: %w", err)
	}
	udps, err := net.Connections("udp")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get UDP connections: %w", err)
	}

	return len(tcps), len(udps), nil
}

func (c *Collector) procRoot() string {
	if c.options.HostProc != "" {
		return c.options.HostProc
	}
	return "/proc"
}

func procNetConnectionsCount(root string) (tcpCount, udpCount int, err error) {
	tcpCount, err = countProcNetFiles(root, "tcp", "tcp6")
	if err != nil {
		return 0, 0, err
	}
	udpCount, err = countProcNetFiles(root, "udp", "udp6")
	if err != nil {
		return 0, 0, err
	}
	return tcpCount, udpCount, nil
}

func countProcNetFiles(root string, names ...string) (int, error) {
	total := 0
	readAny := false
	for _, name := range names {
		count, err := countProcNetFile(filepath.Join(root, "net", name))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, err
		}
		total += count
		readAny = true
	}
	if !readAny {
		return 0, fmt.Errorf("no proc net files found under %s", filepath.Join(root, "net"))
	}
	return total, nil
}

func countProcNetFile(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	header := true
	for scanner.Scan() {
		if header {
			header = false
			continue
		}
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count, scanner.Err()
}

var (
	// 预定义常见的回环和虚拟接口名称
	loopbackNames = map[string]struct{}{
		"br":      {},
		"cni":     {},
		"docker":  {},
		"podman":  {},
		"flannel": {},
		"lo":      {},
		"veth":    {}, // Docker
		"virbr":   {}, // KVM
		"vmbr":    {}, // Proxmox
		"tap":     {},
		"fwbr":    {},
		"fwpr":    {},
	}
)

func (c *Collector) NetworkSpeed() (totalUp, totalDown, upSpeed, downSpeed uint64, err error) {
	includeNICs := parseNICs(c.options.IncludeNICs)
	excludeNICs := parseNICs(c.options.ExcludeNICs)

	// 如果设置了月重置（非0），统计totalUp、totalDown
	if c.options.MonthRotate != 0 {
		if err := c.trafficTracker.Start(); err != nil {
			fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fallbackErr := c.getNetworkSpeedFallback(includeNICs, excludeNICs)
			if fallbackErr != nil {
				return fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fmt.Errorf("start network traffic history: %w; fallback error: %w", err, fallbackErr)
			}
			return fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fmt.Errorf("start network traffic history: %w", err)
		}
		now := uint64(time.Now().Unix())
		resetDay := uint64(netstatic.GetLastResetDate(c.options.MonthRotate, time.Now()).Unix())
		nicStatics, err := c.trafficTracker.TotalTrafficBetween(resetDay, now)
		if err != nil {
			// 如果netstatic失败，回退到原来的方法，并返回额外的错误信息
			fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fallbackErr := c.getNetworkSpeedFallback(includeNICs, excludeNICs)
			if fallbackErr != nil {
				return fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fmt.Errorf("get historical network traffic: %w; fallback error: %w", err, fallbackErr)
			}
			return fallbackUp, fallbackDown, fallbackUpSpeed, fallbackDownSpeed, fmt.Errorf("get historical network traffic: %w", err)
		}

		for interfaceName, stats := range nicStatics {
			if shouldInclude(interfaceName, includeNICs, excludeNICs) {
				totalUp += stats.Tx
				totalDown += stats.Rx
			}
		}

		// 对于实时速度，仍然使用网卡累计计数器差值
		_, _, upSpeed, downSpeed, err = c.getNetworkSpeedFallback(includeNICs, excludeNICs)
		if err != nil {
			return totalUp, totalDown, 0, 0, err
		}

		return totalUp, totalDown, upSpeed, downSpeed, nil
	}

	// 如果没有设置月重置，使用原来的方法
	return c.getNetworkSpeedFallback(includeNICs, excludeNICs)
}

func (c *Collector) getNetworkSpeedFallback(includeNICs, excludeNICs map[string]struct{}) (totalUp, totalDown, upSpeed, downSpeed uint64, err error) {
	totalUp, totalDown, err = collectNetworkTotals(includeNICs, excludeNICs)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	upSpeed, downSpeed = c.updateNetworkSpeedSample(totalUp, totalDown, time.Now())
	return totalUp, totalDown, upSpeed, downSpeed, nil
}

func collectNetworkTotals(includeNICs, excludeNICs map[string]struct{}) (totalUp, totalDown uint64, err error) {
	ioCounters, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	if len(ioCounters) == 0 {
		return 0, 0, fmt.Errorf("no network interfaces found")
	}

	for _, interfaceStats := range ioCounters {
		if shouldInclude(interfaceStats.Name, includeNICs, excludeNICs) {
			totalUp += interfaceStats.BytesSent
			totalDown += interfaceStats.BytesRecv
		}
	}

	return totalUp, totalDown, nil
}

type networkSpeedState struct {
	sync.Mutex
	totalUp   uint64
	totalDown uint64
	sampledAt time.Time
}

func (c *Collector) updateNetworkSpeedSample(totalUp, totalDown uint64, now time.Time) (upSpeed, downSpeed uint64) {
	c.networkSpeed.Lock()
	defer c.networkSpeed.Unlock()

	if c.networkSpeed.sampledAt.IsZero() {
		c.networkSpeed.totalUp = totalUp
		c.networkSpeed.totalDown = totalDown
		c.networkSpeed.sampledAt = now
		return 0, 0
	}

	elapsed := now.Sub(c.networkSpeed.sampledAt).Seconds()
	if elapsed <= 0 {
		return 0, 0
	}

	upDelta := safeCounterDelta(totalUp, c.networkSpeed.totalUp)
	downDelta := safeCounterDelta(totalDown, c.networkSpeed.totalDown)

	c.networkSpeed.totalUp = totalUp
	c.networkSpeed.totalDown = totalDown
	c.networkSpeed.sampledAt = now

	return uint64(float64(upDelta) / elapsed), uint64(float64(downDelta) / elapsed)
}

func safeCounterDelta(current, previous uint64) uint64 {
	if current >= previous {
		return current - previous
	}
	return 0
}

func parseNICs(nics string) map[string]struct{} {
	if nics == "" {
		return nil
	}
	nicSet := make(map[string]struct{})
	for _, nic := range strings.Split(nics, ",") {
		nicSet[strings.TrimSpace(nic)] = struct{}{}
	}
	return nicSet
}

func shouldInclude(nicName string, includeNICs, excludeNICs map[string]struct{}) bool {
	// 默认排除回环接口
	for loopbackName := range loopbackNames {
		if strings.HasPrefix(nicName, loopbackName) {
			return false
		}
	}

	// 如果定义了白名单，则只包括白名单中的接口
	for pattern := range includeNICs {
		if matched, _ := filepath.Match(pattern, nicName); matched {
			return true
		}
	}

	// 如果定义了黑名单，则排除黑名单中的接口
	for pattern := range excludeNICs {
		if matched, _ := filepath.Match(pattern, nicName); matched {
			return false
		}
	}

	return len(includeNICs) == 0 // 如果没有定义白名单，则默认包含所有非回环接口
}

func (c *Collector) InterfaceList() ([]string, error) {
	includeNICs := parseNICs(c.options.IncludeNICs)
	excludeNICs := parseNICs(c.options.ExcludeNICs)
	interfaces := []string{}

	ioCounters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}
	for _, interfaceStats := range ioCounters {
		if shouldInclude(interfaceStats.Name, includeNICs, excludeNICs) {
			interfaces = append(interfaces, interfaceStats.Name)
		}
	}
	return interfaces, nil
}
