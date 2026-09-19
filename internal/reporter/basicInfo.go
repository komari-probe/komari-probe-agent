package reporter

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/internal/protocol/transport"
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

	return tryUploadData(data)
}

func tryUploadData(data map[string]interface{}) error {
	return tryUploadDataWithProtocol(data)
}

func tryUploadDataWithProtocol(data map[string]interface{}) error {
	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/v2/rpc?token=" + flags.Token
	payload := v2.BuildBasicInfoPayload(data)
	body := payload
	compressed := false
	if !flags.DisableCompression {
		if gz, err := transport.GzipBytes(payload); err == nil {
			body = gz
			compressed = true
		}
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	client := connectivity.GetHTTPClientWithPreference(30*time.Second, flags.PreferIPVersion, flags.IgnoreUnsafeCert)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	message := string(respBody)

	if resp.StatusCode != http.StatusOK {
		return &v2.HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: message}
	}
	if len(bytes.TrimSpace(respBody)) > 0 {
		if _, err := v2.ParseResponse(respBody); err != nil {
			return err
		}
	}

	return nil
}
