package reporter

import (
	"encoding/json"
	"fmt"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
)

type report struct {
	CPU         cpuReport         `json:"cpu"`
	Ram         usageReport       `json:"ram"`
	Swap        usageReport       `json:"swap"`
	Load        loadReport        `json:"load"`
	Disk        usageReport       `json:"disk"`
	Network     networkReport     `json:"network"`
	Connections connectionsReport `json:"connections"`
	GPU         any               `json:"gpu,omitempty"`
	Uptime      uint64            `json:"uptime"`
	Process     int               `json:"process"`
	Message     string            `json:"message"`
}

type cpuReport struct {
	Usage float64 `json:"usage"`
}

type usageReport struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

type loadReport struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

type networkReport struct {
	Up        uint64 `json:"up"`
	Down      uint64 `json:"down"`
	TotalUp   uint64 `json:"totalUp"`
	TotalDown uint64 `json:"totalDown"`
}

type connectionsReport struct {
	TCP int `json:"tcp"`
	UDP int `json:"udp"`
}

type gpuModelsReport struct {
	Models []string `json:"models"`
}

type gpuReport struct {
	Count        int               `json:"count"`
	AverageUsage float64           `json:"average_usage"`
	DetailedInfo []gpuDeviceReport `json:"detailed_info"`
}

type gpuDeviceReport struct {
	Name        string  `json:"name"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUsed  uint64  `json:"memory_used"`
	Utilization float64 `json:"utilization"`
	Temperature uint64  `json:"temperature"`
}

func (r *Reporter) GenerateReport() ([]byte, error) {
	message := ""
	data := report{}

	cpu := collector.CPU()
	cpuUsage := cpu.CPUUsage
	if cpuUsage <= 0.001 {
		cpuUsage = 0.001
	}
	data.CPU = cpuReport{Usage: cpuUsage}

	ram := r.collector.RAM()
	data.Ram = usageReport{Total: ram.Total, Used: ram.Used}

	swap := collector.Swap()
	data.Swap = usageReport{Total: swap.Total, Used: swap.Used}
	load := collector.Load()
	data.Load = loadReport{Load1: load.Load1, Load5: load.Load5, Load15: load.Load15}

	disk := r.collector.Disk()
	data.Disk = usageReport{Total: disk.Total, Used: disk.Used}

	totalUp, totalDown, networkUp, networkDown, err := r.collector.NetworkSpeed()
	if err != nil {
		message += fmt.Sprintf("failed to get network speed: %v\n", err)
	}
	data.Network = networkReport{Up: networkUp, Down: networkDown, TotalUp: totalUp, TotalDown: totalDown}

	tcpCount, udpCount, err := r.collector.ConnectionsCount()
	if err != nil {
		message += fmt.Sprintf("failed to get connections: %v\n", err)
	}
	data.Connections = connectionsReport{TCP: tcpCount, UDP: udpCount}

	uptime, err := collector.Uptime()
	if err != nil {
		message += fmt.Sprintf("failed to get uptime: %v\n", err)
	}
	data.Uptime = uptime

	data.Process = r.collector.ProcessCount()

	// GPU监控 - 根据标志决定详细程度
	if r.options.EnableGPU {
		// 详细GPU监控模式
		gpuInfo, err := r.collector.GPUDevices()
		if err != nil {
			message += fmt.Sprintf("failed to get detailed GPU info: %v\n", err)
			// 降级到基础GPU信息
			gpuNames, nameErr := r.collector.GPUModelNames()
			if nameErr == nil && len(gpuNames) > 0 {
				data.GPU = gpuModelsReport{Models: gpuNames}
			}
		} else if len(gpuInfo) > 0 {
			// 成功获取详细信息
			gpuData := make([]gpuDeviceReport, len(gpuInfo))
			totalGPUUsage := 0.0

			for i, info := range gpuInfo {
				gpuData[i] = gpuDeviceReport{
					Name:        info.Name,
					MemoryTotal: info.MemoryTotal,
					MemoryUsed:  info.MemoryUsed,
					Utilization: info.Utilization,
					Temperature: info.Temperature,
				}
				totalGPUUsage += info.Utilization
			}

			avgGPUUsage := totalGPUUsage / float64(len(gpuInfo))
			data.GPU = gpuReport{Count: len(gpuInfo), AverageUsage: avgGPUUsage, DetailedInfo: gpuData}
		}
	}
	// 基础模式下，GPU信息已在basicInfo中处理

	data.Message = message

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal performance report: %w", err)
	}
	return payload, nil
}
