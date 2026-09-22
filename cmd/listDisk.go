package cmd

import (
	"fmt"
	log "github.com/sonar-probe/sonar-agent/internal/logging"
	"text/tabwriter"

	"github.com/sonar-probe/sonar-agent/internal/collector"
	"github.com/sonar-probe/sonar-agent/internal/config"
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
			table := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "Mountpoint\tFilesystem")
			for _, part := range dl {
				fmt.Fprintf(table, "%s\t%s\n", part.Mountpoint, part.Fstype)
			}
			if err := table.Flush(); err != nil {
				log.Println("Failed to write disk list:", err)
				return
			}
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
