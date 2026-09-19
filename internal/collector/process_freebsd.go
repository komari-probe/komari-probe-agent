//go:build freebsd
// +build freebsd

package collector

import (
	"os/exec"
	"strings"
)

// processCountFreeBSD counts processes using the `ps` command
func processCount(_ string) (count int) {
	cmd := exec.Command("ps", "-ax")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Count the number of lines in the output, excluding the header line
	lines := strings.Split(string(output), "\n")
	return len(lines) - 1
}
