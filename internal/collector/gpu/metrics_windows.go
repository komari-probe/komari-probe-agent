//go:build windows

package gpu

import "errors"

// ModelNames 获取GPU型号信息 (Windows: 仅支持 NVIDIA)
func ModelNames() ([]string, error) {
	smi := &nvidiaSMI{}
	if err := smi.start(); err != nil {
		return nil, err
	}
	return smi.modelNames()
}

// Devices returns NVIDIA GPU metrics on Windows.
func Devices() ([]Device, error) {
	smi := &nvidiaSMI{}
	if err := smi.start(); err != nil {
		return nil, err
	}

	data, err := smi.devices()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, errors.New("no NVIDIA GPU detected")
	}

	gpuInfos := make([]Device, len(data))
	for i, nvidiaInfo := range data {
		gpuInfos[i] = Device{
			Name:        nvidiaInfo.Name,
			MemoryTotal: nvidiaInfo.MemoryTotal,
			MemoryUsed:  nvidiaInfo.MemoryUsed,
			Utilization: nvidiaInfo.Utilization,
			Temperature: nvidiaInfo.Temperature,
		}
	}

	return gpuInfos, nil
}
