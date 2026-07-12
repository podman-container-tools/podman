package main

import (
	pluginapi "github.com/docker/go-plugins-helpers/volume"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "create a volume",
	Long:  `Create a volume in the volume plugin listening on --sock-name`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return createVol(config.sockName, args[0])
	},
}

func createVol(sockName, volName string) error {
	plugin, err := getPlugin(sockName)
	if err != nil {
		return err
	}
	createReq := new(pluginapi.CreateRequest)
	createReq.Name = volName
	return plugin.CreateVolume(createReq)
}
