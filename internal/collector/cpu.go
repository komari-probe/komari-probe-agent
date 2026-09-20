package collector

import (
	"bufio"
	"os"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
)

type CPUInfo struct {
	CPUName          string  `json:"cpu_name"`
	CPUArchitecture  string  `json:"cpu_architecture"`
	CPUCores         int     `json:"cpu_cores"`
	CPUPhysicalCores int     `json:"cpu_physical_cores"`
	CPUUsage         float64 `json:"cpu_usage"`
}

func CPU() CPUInfo {
	cpuInfo := CPUStaticInfo()

	percentages, err := cpu.Percent(0, false)
	if err == nil && len(percentages) > 0 {
		cpuInfo.CPUUsage = percentages[0]
	}

	return cpuInfo
}

func CPUStaticInfo() CPUInfo {
	cpuInfo := CPUInfo{
		CPUName:          "Unknown",
		CPUArchitecture:  runtime.GOARCH,
		CPUCores:         1,
		CPUPhysicalCores: 0, // 为兼容旧版 agent，0 表示未上报或未知，避免与实际核心数混淆
		CPUUsage:         0.0,
	}

	// 优先使用 gopsutil 获取 CPU 信息，避免触发 lscpu 在部分内核上的 lockdown 日志刷屏。
	info, err := cpu.Info()
	if err == nil && len(info) > 0 {
		cpuInfo.CPUName = strings.TrimSpace(info[0].ModelName)
		if cpuInfo.CPUName == "" {
			if info[0].VendorID != "" || info[0].Family != "" {
				cpuInfo.CPUName = strings.TrimSpace(info[0].VendorID + " " + info[0].Family)
			}
		}
	}

	if cpuInfo.CPUName == "Unknown" {
		name, err := readCPUNameFromProc()
		if err == nil && name != "" {
			cpuInfo.CPUName = strings.TrimSpace(name)
		}
	}

	cores, err := cpu.Counts(true)
	if err == nil && cores > 0 {
		cpuInfo.CPUCores = cores
	}

	physicalCores, err := cpu.Counts(false)
	if err == nil && physicalCores > 0 {
		cpuInfo.CPUPhysicalCores = physicalCores
	}

	return cpuInfo
}

// readCPUNameFromProc 从 /proc/cpuinfo 读取 CPU 名称
func readCPUNameFromProc() (string, error) {
	file, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Model\t") || strings.HasPrefix(line, "Hardware\t") || strings.HasPrefix(line, "Processor\t") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", scanner.Err()
}
