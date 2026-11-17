package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list all volumes",
	Long:  `List all volumes from the volume plugin listening on --sock-name`,
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return listVol(config.sockName)
	},
}

func listVol(sockName string) error {
	plugin, err := getPlugin(sockName)
	if err != nil {
		return err
	}
	vols, err := plugin.ListVolumes()
	if err != nil {
		return err
	}
	for _, vol := range vols {
		fmt.Println(vol.Name)
	}
	return nil
}
