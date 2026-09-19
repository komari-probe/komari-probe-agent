//go:build !windows
// +build !windows

package collector

import (
	"os"
	"strconv"
)

// processCountLinux counts processes by reading /proc directory
func processCount(hostProc string) (count int) {
	procDir := "/proc"

	if hostProc != "" {
		if info, err := os.Stat(hostProc); err == nil && info.IsDir() {
			procDir = hostProc
		}
	}

	entries, err := os.ReadDir(procDir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if _, err := strconv.ParseInt(entry.Name(), 10, 64); err == nil {
			//if _, err := filepath.ParseInt(entry.Name(), 10, 64); err == nil {
			count++
		}
	}

	return count
}
