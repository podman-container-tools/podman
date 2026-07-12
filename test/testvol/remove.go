package main

import (
	pluginapi "github.com/docker/go-plugins-helpers/volume"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove NAME",
	Short: "remove a volume",
	Long:  `Remove a volume in the volume plugin listening on --sock-name`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return removeVol(config.sockName, args[0])
	},
}

func removeVol(sockName, volName string) error {
	plugin, err := getPlugin(sockName)
	if err != nil {
		return err
	}
	removeReq := new(pluginapi.RemoveRequest)
	removeReq.Name = volName
	return plugin.RemoveVolume(removeReq)
}
