//go:build darwin
// +build darwin

package gpu

import (
	"os/exec"
	"strings"
)

// Name returns the name of the GPU on Darwin (macOS)
func Name() string {
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}

	lines := strings.Split(string(output), "\n")
	var names []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Chipset Model:") {
			names = append(names, strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:")))
		}
	}

	return formatNameList(names)
}
