package cmd

import (
	"fmt"
	"log"
	"text/tabwriter"

	"github.com/komari-probe/komari-probe-agent/internal/collector"
	"github.com/komari-probe/komari-probe-agent/internal/config"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/spf13/cobra"
)

func newListDiskCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list-disk",
		Short: "List all physical disks",
		Long:  `List all physical disks`,
		Run: func(cmd *cobra.Command, args []string) {
			dl, err := disk.Partitions(true)
			if err != nil {
				log.Println("Failed to get disk partitions:", err)
				return
			}
			log.Println("All Disk Partitions:")
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Mountpoint\tFstype")
			for _, part := range dl {
				fmt.Fprintf(w, "%s\t%s\n", part.Mountpoint, part.Fstype)
			}
			_ = w.Flush()
			hostCollector := collector.New(collector.Options{IncludeMountpoints: cfg.IncludeMountpoints})
			diskList, err := hostCollector.DiskList()
			if err != nil {
				log.Println("Failed to get disk list:", err)
				return
			}
			log.Println("Monitoring Mountpoints:", diskList)
		},
	}
}
