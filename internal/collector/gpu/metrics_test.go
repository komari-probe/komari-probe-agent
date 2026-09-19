package gpu

import (
	"testing"
)

func TestDetailedGPUDetection(t *testing.T) {
	models, err := ModelNames()
	if err != nil {
		t.Logf("Detailed GPU detection failed (may be normal on non-Linux or non-GPU systems): %v", err)
		return
	}

	t.Logf("Detected GPUs: %v", models)

	if len(models) > 0 {
		// 测试详细信息获取
		detailedInfo, err := Devices()
		if err != nil {
			t.Logf("GPU detailed info collection failed: %v", err)
		} else {
			for i, info := range detailedInfo {
				t.Logf("GPU %d: %s - Memory: %dMB/%dMB, Usage: %.1f%%, Temp: %d°C",
					i, info.Name, info.MemoryUsed, info.MemoryTotal, info.Utilization, info.Temperature)
			}
		}
	}
}

func TestDevice(t *testing.T) {
	detailedInfo, err := Devices()
	if err != nil {
		t.Logf("GPU detailed info test failed (may be normal): %v", err)
		return
	}

	if len(detailedInfo) == 0 {
		t.Log("No detailed GPU info available")
		return
	}

	for i, info := range detailedInfo {
		t.Logf("GPU %d Details:", i)
		t.Logf("  Name: %s", info.Name)
		t.Logf("  Memory Total: %d MB", info.MemoryTotal)
		t.Logf("  Memory Used: %d MB", info.MemoryUsed)
		//t.Logf("  Memory Free: %d MB", info.MemoryFree)
		t.Logf("  Utilization: %.1f%%", info.Utilization)
		t.Logf("  Temperature: %d°C", info.Temperature)

		// 验证数据的合理性
		//if info.MemoryTotal > 0 && info.MemoryUsed+info.MemoryFree != info.MemoryTotal {
		//	t.Logf("Warning: Memory usage calculation may be inconsistent for %s", info.Name)
		//}

		if info.Utilization < 0 || info.Utilization > 100 {
			t.Errorf("Invalid utilization value for %s: %.1f%%", info.Name, info.Utilization)
		}
	}
}
