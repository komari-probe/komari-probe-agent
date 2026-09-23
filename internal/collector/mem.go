package collector

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
)

type RAMInfo struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Mode  string
}

type ProcMemInfo struct {
	MemTotal     uint64
	MemFree      uint64
	MemAvailable uint64
	Buffers      uint64
	Cached       uint64
	SwapTotal    uint64
	SwapFree     uint64
	SwapCached   uint64
	Shmem        uint64
	SReclaimable uint64
	Zswap        uint64
	Zswapped     uint64
}

// readProcMeminfo reads /proc/meminfo and returns a filled ProcMemInfo struct
func ReadProcMeminfo() (*ProcMemInfo, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseProcMeminfo(file)
}

func parseProcMeminfo(reader io.Reader) (*ProcMemInfo, error) {
	info := &ProcMemInfo{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSuffix(parts[0], ":")
		valStr := parts[1]
		val, err := strconv.ParseUint(valStr, 10, 64)
		if err != nil {
			continue
		}
		val *= 1024 // Convert kB to bytes

		switch key {
		case "MemTotal":
			info.MemTotal = val
		case "MemFree":
			info.MemFree = val
		case "MemAvailable":
			info.MemAvailable = val
		case "Buffers":
			info.Buffers = val
		case "Cached":
			info.Cached = val
		case "SwapTotal":
			info.SwapTotal = val
		case "SwapFree":
			info.SwapFree = val
		case "SwapCached":
			info.SwapCached = val
		case "Shmem":
			info.Shmem = val
		case "SReclaimable":
			info.SReclaimable = val
		case "Zswap":
			info.Zswap = val
		case "Zswapped":
			info.Zswapped = val
		}
	}
	return info, scanner.Err()
}

func MemoryHtopLike() RAMInfo {
	ramInfo := RAMInfo{Mode: "htoplike"}
	if runtime.GOOS == "linux" {
		info, err := ReadProcMeminfo()
		if err == nil && info.MemTotal > 0 {
			ramInfo.Total = info.MemTotal
			// htop logic:
			// usedDiff = free + cached + sreclaimable + buffers
			usedDiff := info.MemFree + info.Cached + info.SReclaimable + info.Buffers

			if info.MemTotal >= usedDiff {
				ramInfo.Used = info.MemTotal - usedDiff
			} else {
				ramInfo.Used = info.MemTotal - info.MemFree
			}
			ramInfo.Used += info.Shmem
			return ramInfo
		}
	}
	return ramInfo
}

// MemoryFromAvailable computes used memory the way modern `free`/`free -h`
// does: total minus the kernel-reported MemAvailable. MemAvailable already
// accounts for reclaimable caches/buffers (and the kernel's own reclaim
// overhead), so unlike the older free+cached+buffers heuristic in
// MemoryHtopLike, it matches what `free -h` actually prints.
func MemoryFromAvailable() RAMInfo {
	ramInfo := RAMInfo{Mode: "available"}
	if runtime.GOOS == "linux" {
		info, err := ReadProcMeminfo()
		if err == nil {
			return ramInfoFromAvailable(info)
		}
	}
	return ramInfo
}

func ramInfoFromAvailable(info *ProcMemInfo) RAMInfo {
	ramInfo := RAMInfo{Mode: "available"}
	if info.MemTotal == 0 || info.MemAvailable == 0 {
		return ramInfo
	}
	ramInfo.Total = info.MemTotal
	if info.MemAvailable < info.MemTotal {
		ramInfo.Used = info.MemTotal - info.MemAvailable
	}
	return ramInfo
}

func MemoryGopsutil() RAMInfo {
	ramInfo := RAMInfo{Mode: "gopsutil"}
	v, err := mem.VirtualMemory()
	if err == nil {
		ramInfo.Total = v.Total
		ramInfo.Used = v.Total - v.Available
	}
	return ramInfo
}

// MemoryFromFree returns the values reported by the free command when it is
// available on Linux or FreeBSD.
func MemoryFromFree() RAMInfo {
	ramInfo := RAMInfo{Mode: "free"}

	// Only works on Linux/Unix systems
	if runtime.GOOS != "linux" && runtime.GOOS != "freebsd" {
		return ramInfo
	}

	// Execute 'free -b' command to get memory in bytes
	cmd := exec.Command("free", "-b")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ramInfo
	}

	// Parse the output
	scanner := bufio.NewScanner(&out)
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Skip the header line
		if lineNum == 1 {
			continue
		}

		// Parse the "Mem:" line
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			// Format: Mem: total used free shared buff/cache available
			if len(fields) >= 3 {
				total, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					ramInfo.Total = total
				}

				used, err := strconv.ParseUint(fields[2], 10, 64)
				if err == nil {
					ramInfo.Used = used
				}
			}
			break
		}
	}

	return ramInfo
}

func (c *Collector) RAM() RAMInfo {
	if c.options.MemoryIncludeCache {
		v, err := mem.VirtualMemory()
		if err != nil {
			return RAMInfo{}
		}
		return RAMInfo{
			Total: v.Total,
			Used:  v.Total - v.Free,
			Mode:  "includeCache",
		}
	}

	if c.options.MemoryReportRawUsed {
		return MemoryHtopLike()
	}

	if runtime.GOOS == "linux" {
		// MemAvailable-based accounting matches `free -h`; only kernels older
		// than 3.14 lack it, in which case fall back to the htop-style heuristic.
		a := MemoryFromAvailable()
		if a.Total > 0 {
			return a
		}
		h := MemoryHtopLike()
		if h.Total > 0 {
			return h
		}
	}

	// Default fallback
	return MemoryGopsutil()
}

func Swap() RAMInfo {
	swapInfo := RAMInfo{}

	if runtime.GOOS == "linux" {
		info, err := ReadProcMeminfo()
		if err == nil {
			swapInfo.Total = info.SwapTotal
			// used = total - free - cached
			// Check for underflow
			usedDeductions := info.SwapFree + info.SwapCached
			if info.SwapTotal >= usedDeductions {
				swapInfo.Used = info.SwapTotal - usedDeductions
			} else {
				swapInfo.Used = info.SwapTotal - info.SwapFree
			}
			return swapInfo
		}
	}

	s, err := mem.SwapMemory()
	if err != nil {
		return swapInfo
	}
	swapInfo.Total = s.Total
	swapInfo.Used = s.Used
	return swapInfo
}
