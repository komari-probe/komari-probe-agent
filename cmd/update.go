package cmd

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/komari-probe/komari-probe-agent/internal/update"
	"github.com/komari-probe/komari-probe-agent/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Download and install a newer Agent release",
		Long:  "Checks the official Komari Probe Agent GitHub release, verifies its SHA-256 checksum, and replaces this executable. It never enables automatic updates.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			if cfg.IgnoreUnsafeCert {
				transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- explicit existing CLI option
			}
			client := update.NewClient(&http.Client{Transport: transport, Timeout: 60 * time.Second})
			result, err := client.Run(version.CurrentVersion)
			if err != nil {
				return err
			}
			if !result.Updated {
				fmt.Printf("Komari Probe Agent is already up to date (%s).\n", result.CurrentVersion)
				return nil
			}
			fmt.Printf("Updated Komari Probe Agent from %s to %s. Restart the Agent service to run the new version.\n", result.CurrentVersion, result.TargetVersion)
			return nil
		},
	}
}
