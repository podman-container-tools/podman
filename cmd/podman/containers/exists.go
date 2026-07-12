package containers

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	containerExistsDescription = `If the named container exists in local storage, podman container exists exits with 0, otherwise the exit code will be 1.`

	existsCommand = &cobra.Command{
		Use:   "exists [options] CONTAINER",
		Short: "Check if a container exists in local storage",
		Long:  containerExistsDescription,
		Example: `podman container exists --external containerID
podman container exists myctr || podman run --name myctr [etc...]`,
		RunE:              exists,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteContainers,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCommand,
		Parent:  containerCmd,
	})
	flags := existsCommand.Flags()
	flags.Bool("external", false, "Check external storage containers as well as Podman containers")
}

func exists(cmd *cobra.Command, args []string) error {
	external, err := cmd.Flags().GetBool("external")
	if err != nil {
		return err
	}
	options := entities.ContainerExistsOptions{
		External: external,
	}
	response, err := registry.ContainerEngine().ContainerExists(context.Background(), strings.TrimPrefix(args[0], "/"), options)
	if err != nil {
		return err
	}
	if !response.Value {
		registry.SetExitCode(1)
	}
	return nil
}
