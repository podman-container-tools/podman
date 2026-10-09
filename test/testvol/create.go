package main

import (
	"context"

	pluginapi "github.com/docker/go-plugins-helpers/volume"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create NAME",
	Short: "create a volume",
	Long:  `Create a volume in the volume plugin listening on --sock-name`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return createVol(cmd.Context(), config.sockName, args[0])
	},
}

func createVol(ctx context.Context, sockName, volName string) error {
	plugin, err := getPlugin(ctx, sockName)
	if err != nil {
		return err
	}
	createReq := new(pluginapi.CreateRequest)
	createReq.Name = volName
	return plugin.CreateVolume(ctx, createReq)
}
