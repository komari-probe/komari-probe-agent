package gpu

// Modified from https://github.com/influxdata/telegraf/blob/master/plugins/inputs/amd_rocm_smi/amd_rocm_smi.go
// Original License: MIT

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type rocmSMI struct {
	BinPath string
	data    []byte
}

// amdDevice AMD GPU详细信息
type amdDevice struct {
	Name        string  // GPU型号
	MemoryTotal uint64  // 总显存 (字节)
	MemoryUsed  uint64  // 已用显存 (字节)
	Utilization float64 // GPU使用率 (0-100)
	Temperature uint64  // 温度 (摄氏度)
}

func (rsmi *rocmSMI) modelNames() ([]string, error) {
	return rsmi.gatherModel()
}

// devices 获取详细GPU信息
func (rsmi *rocmSMI) devices() ([]amdDevice, error) {
	return rsmi.gatherDetailedInfo()
}

func (rsmi *rocmSMI) start() error {
	if _, err := os.Stat(rsmi.BinPath); os.IsNotExist(err) {
		binPath, err := exec.LookPath("rocm-smi")
		if err != nil {
			return errors.New("rocm-smi tool not found")
		}
		rsmi.BinPath = binPath
	}

	rsmi.data = rsmi.pollrocmSMI()
	return nil
}

func (rsmi *rocmSMI) pollrocmSMI() []byte {
	cmd := exec.Command(rsmi.BinPath, "--showallinfo", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	return output
}

func (rsmi *rocmSMI) gatherModel() ([]string, error) {
	var data map[string]any
	var models []string

	if err := json.Unmarshal(rsmi.data, &data); err != nil {
		return nil, err
	}

	// 解析JSON结构获取GPU型号
	for key, value := range data {
		if strings.HasPrefix(key, "card") {
			if cardData, ok := value.(map[string]any); ok {
				if name, exists := cardData["Card series"]; exists {
					if nameStr, ok := name.(string); ok && nameStr != "" {
						models = append(models, nameStr)
					}
				}
			}
		}
	}

	return models, nil
}

func (rsmi *rocmSMI) gatherDetailedInfo() ([]amdDevice, error) {
	if rsmi.data == nil {
		return nil, errors.New("no data available")
	}

	var data map[string]any
	var gpuInfos []amdDevice

	if err := json.Unmarshal(rsmi.data, &data); err != nil {
		return nil, err
	}

	// 解析每个GPU卡的详细信息
	for key, value := range data {
		if strings.HasPrefix(key, "card") {
			if cardData, ok := value.(map[string]any); ok {
				gpuInfo := amdDevice{}

				// 获取GPU名称
				if name, exists := cardData["Card series"]; exists {
					if nameStr, ok := name.(string); ok {
						gpuInfo.Name = nameStr
					}
				}

				// 获取使用率
				if utilizationData, exists := cardData["GPU use (%)"]; exists {
					if utilizationStr, ok := utilizationData.(string); ok {
						if usage, err := parseAMDPercentage(utilizationStr); err == nil {
							gpuInfo.Utilization = usage
						}
					}
				}

				// 获取显存信息
				if memUsedData, exists := cardData["VRAM Total Used Memory (B)"]; exists {
					if memUsedStr, ok := memUsedData.(string); ok {
						if memUsed, err := parseAMDMemoryBytes(memUsedStr); err == nil {
							gpuInfo.MemoryUsed = memUsed
						}
					}
				}

				if memTotalData, exists := cardData["VRAM Total Memory (B)"]; exists {
					if memTotalStr, ok := memTotalData.(string); ok {
						if memTotal, err := parseAMDMemoryBytes(memTotalStr); err == nil {
							gpuInfo.MemoryTotal = memTotal
						}
					}
				}

				// 获取温度信息
				if tempData, exists := cardData["Temperature (Sensor junction) (C)"]; exists {
					if tempStr, ok := tempData.(string); ok {
						if temp, err := parseAMDTemperature(tempStr); err == nil {
							gpuInfo.Temperature = temp
						}
					}
				}

				gpuInfos = append(gpuInfos, gpuInfo)
			}
		}
	}

	return gpuInfos, nil
}

// 解析AMD百分比值 (例如 "25" -> 25.0)
func parseAMDPercentage(value string) (float64, error) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.TrimSuffix(cleaned, "%")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return 0.0, nil
	}

	result, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0.0, err
	}

	return result, nil
}

// 解析AMD显存字节 (例如 "1073741824" -> 1073741824字节)
func parseAMDMemoryBytes(value string) (uint64, error) {
	cleaned := strings.TrimSpace(value)

	if cleaned == "" {
		return 0, nil
	}

	bytes, err := strconv.ParseUint(cleaned, 10, 64)
	if err != nil {
		return 0, err
	}

	// 直接返回字节数
	return bytes, nil
}

// 解析AMD温度值 (例如 "65" -> 65)
func parseAMDTemperature(value string) (uint64, error) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.TrimSuffix(cleaned, "C")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return 0, nil
	}

	result, err := strconv.ParseUint(cleaned, 10, 64)
	if err != nil {
		return 0, err
	}

	return result, nil
}
