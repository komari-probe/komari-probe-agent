package reporter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/version"
)

func (r *Reporter) RunStaticInfoReporter() {
	ticker := time.NewTicker(time.Duration(r.options.InfoReportInterval) * time.Minute)
	for range ticker.C {
		err := r.uploadBasicInfo()
		if err != nil {
			log.Println("Error uploading basic info:", err)
		}
	}
}
func (r *Reporter) UpdateBasicInfo() {
	err := r.uploadBasicInfo()
	if err != nil {
		log.Println("Error uploading basic info:", err)
	} else {
		log.Println("Basic info uploaded successfully")
	}
}
func (r *Reporter) uploadBasicInfo() error {
	cpu := collector.CPUStaticInfo()

	osname := collector.OSName()
	kernelVersion := collector.KernelVersion()
	ipv4, ipv6 := r.collector.IPAddresses()

	data := map[string]any{
		"cpu_name":           cpu.CPUName,
		"cpu_cores":          cpu.CPUCores,
		"cpu_physical_cores": cpu.CPUPhysicalCores,
		"arch":               cpu.CPUArchitecture,
		"os":                 osname,
		"kernel_version":     kernelVersion,
		"ipv4":               ipv4,
		"ipv6":               ipv6,
		"mem_total":          r.collector.RAM().Total,
		"swap_total":         collector.Swap().Total,
		"disk_total":         r.collector.Disk().Total,
		"gpu_name":           r.collector.GPUName(),
		"virtualization":     collector.Virtualization(),
		"version":            version.CurrentVersion,
	}

	payload, err := buildBasicInfoPayload(data)
	if err != nil {
		return fmt.Errorf("build basic-info payload: %w", err)
	}
	return r.postAndValidateRPC(context.Background(), payload, 30*time.Second)
}
