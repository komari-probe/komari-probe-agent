//go:build linux

package gpu

import "errors"

const (
	vendorAMD = iota + 1
	vendorNVIDIA
)

func detectVendor() uint8 {
	_, err := getNvidiaDetailedStat()
	if err != nil {
		return vendorAMD
	} else {
		return vendorNVIDIA
	}
}

func getNvidiaDetailedStat() ([]float64, error) {
	smi := &nvidiaSMI{
		BinPath: "/usr/bin/nvidia-smi",
	}
	err1 := smi.start()
	if err1 != nil {
		return nil, err1
	}
	data, err2 := smi.usage()
	if err2 != nil {
		return nil, err2
	}
	return data, nil
}

func getNvidiaDetailedHost() ([]string, error) {
	smi := &nvidiaSMI{
		BinPath: "/usr/bin/nvidia-smi",
	}
	err := smi.start()
	if err != nil {
		return nil, err
	}
	data, err := smi.modelNames()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func getAMDDetailedHost() ([]string, error) {
	if data, err := getAMDROCmDetailedHost(); err == nil && len(data) > 0 {
		return data, nil
	}
	return getAMDSysfsDetailedHost()
}

// ModelNames 获取GPU型号信息
func ModelNames() ([]string, error) {
	var gi []string
	var err error

	switch detectVendor() {
	case vendorAMD:
		gi, err = getAMDDetailedHost()
	case vendorNVIDIA:
		gi, err = getNvidiaDetailedHost()
	default:
		return nil, errors.New("invalid vendor")
	}

	if err != nil {
		return nil, err
	}

	return gi, nil
}

// Devices returns metrics for every detected GPU device.
func Devices() ([]Device, error) {
	var gpuInfos []Device
	var err error

	switch detectVendor() {
	case vendorAMD:
		gpuInfos, err = getAMDDetailedInfo()
	case vendorNVIDIA:
		gpuInfos, err = getNvidiaDetailedInfo()
	default:
		return nil, errors.New("invalid vendor")
	}

	if err != nil {
		return nil, err
	}

	return gpuInfos, nil
}

func getNvidiaDetailedInfo() ([]Device, error) {
	smi := &nvidiaSMI{
		BinPath: "/usr/bin/nvidia-smi",
	}
	err := smi.start()
	if err != nil {
		return nil, err
	}

	data, err := smi.devices()
	if err != nil {
		return nil, err
	}

	var gpuInfos []Device
	for _, nvidiaInfo := range data {
		gpuInfo := Device{
			Name:        nvidiaInfo.Name,
			MemoryTotal: nvidiaInfo.MemoryTotal,
			MemoryUsed:  nvidiaInfo.MemoryUsed,
			Utilization: nvidiaInfo.Utilization,
			Temperature: nvidiaInfo.Temperature,
		}
		gpuInfos = append(gpuInfos, gpuInfo)
	}

	return gpuInfos, nil
}

func getAMDDetailedInfo() ([]Device, error) {
	if gpuInfos, err := getAMDROCmDetailedInfo(); err == nil && len(gpuInfos) > 0 {
		return gpuInfos, nil
	}
	return getAMDSysfsDetailedInfo()
}

func getAMDROCmDetailedHost() ([]string, error) {
	rsmi := &rocmSMI{
		BinPath: "/opt/rocm/bin/rocm-smi",
	}
	if err := rsmi.start(); err != nil {
		return nil, err
	}
	return rsmi.modelNames()
}

func getAMDROCmDetailedInfo() ([]Device, error) {
	rsmi := &rocmSMI{
		BinPath: "/opt/rocm/bin/rocm-smi",
	}
	if err := rsmi.start(); err != nil {
		return nil, err
	}

	data, err := rsmi.devices()
	if err != nil {
		return nil, err
	}

	var gpuInfos []Device
	for _, amdInfo := range data {
		gpuInfos = append(gpuInfos, Device{
			Name:        amdInfo.Name,
			MemoryTotal: amdInfo.MemoryTotal,
			MemoryUsed:  amdInfo.MemoryUsed,
			Utilization: amdInfo.Utilization,
			Temperature: amdInfo.Temperature,
		})
	}
	return gpuInfos, nil
}
