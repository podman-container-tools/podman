package main

import (
	"context"

	pluginapi "github.com/docker/go-plugins-helpers/volume"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove NAME",
	Short: "remove a volume",
	Long:  `Remove a volume in the volume plugin listening on --sock-name`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return removeVol(cmd.Context(), config.sockName, args[0])
	},
}

func removeVol(ctx context.Context, sockName, volName string) error {
	plugin, err := getPlugin(ctx, sockName)
	if err != nil {
		return err
	}
	removeReq := new(pluginapi.RemoveRequest)
	removeReq.Name = volName
	return plugin.RemoveVolume(ctx, removeReq)
}
