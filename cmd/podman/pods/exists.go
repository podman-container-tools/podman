package pods

import (
	"context"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var (
	podExistsDescription = `If the named pod exists in local storage, podman pod exists exits with 0, otherwise the exit code will be 1.`

	existsCommand = &cobra.Command{
		Use:               "exists POD",
		Short:             "Check if a pod exists in local storage",
		Long:              podExistsDescription,
		RunE:              exists,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompletePods,
		Example: `podman pod exists podID
podman pod exists mypod || podman pod create --name mypod`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCommand,
		Parent:  podCmd,
	})
}

func exists(_ *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().PodExists(context.Background(), args[0])
	if err != nil {
		return err
	}
	if !response.Value {
		registry.SetExitCode(1)
	}
	return nil
}
