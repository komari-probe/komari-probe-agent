package main

import (
	"os"

	"github.com/komari-probe/komari-probe-agent/cmd"
)

func main() {
	cmd.Execute()
	os.Exit(0)
}
