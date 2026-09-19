package reporter

import (
	"context"
	"log"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	v2 "github.com/komari-probe/komari-probe-agent/internal/protocol/v2"
	"github.com/komari-probe/komari-probe-agent/internal/version"

	"github.com/komari-probe/komari-probe-agent/internal/config"
)

var flags = config.GlobalConfig

func DoUploadBasicInfoWorks() {
	ticker := time.NewTicker(time.Duration(flags.InfoReportInterval) * time.Minute)
	for range ticker.C {
		err := uploadBasicInfo()
		if err != nil {
			log.Println("Error uploading basic info:", err)
		}
	}
}
func UpdateBasicInfo() {
	err := uploadBasicInfo()
	if err != nil {
		log.Println("Error uploading basic info:", err)
	} else {
		log.Println("Basic info uploaded successfully")
	}
}
func uploadBasicInfo() error {
	cpu := collector.CpuStaticInfo()

	osname := collector.OSName()
	kernelVersion := collector.KernelVersion()
	ipv4, ipv6, _ := collector.GetIPAddress()

	data := map[string]interface{}{
		"cpu_name":           cpu.CPUName,
		"cpu_cores":          cpu.CPUCores,
		"cpu_physical_cores": cpu.CPUPhysicalCores,
		"arch":               cpu.CPUArchitecture,
		"os":                 osname,
		"kernel_version":     kernelVersion,
		"ipv4":               ipv4,
		"ipv6":               ipv6,
		"mem_total":          collector.Ram().Total,
		"swap_total":         collector.Swap().Total,
		"disk_total":         collector.Disk().Total,
		"gpu_name":           collector.GpuName(),
		"virtualization":     collector.Virtualized(),
		"version":            version.CurrentVersion,
	}

	return postAndValidateRPC(context.Background(), v2.BuildBasicInfoPayload(data), 30*time.Second)
}
