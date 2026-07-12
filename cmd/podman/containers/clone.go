package containers

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	cloneDescription = `Creates a copy of an existing container.`

	containerCloneCommand = &cobra.Command{
		Use:               "clone [options] CONTAINER NAME IMAGE",
		Short:             "Clone an existing container",
		Long:              cloneDescription,
		RunE:              clone,
		Args:              cobra.RangeArgs(1, 3),
		ValidArgsFunction: common.AutocompleteClone,
		Example:           `podman container clone container_name new_name image_name`,
	}
)

var ctrClone entities.ContainerCloneOptions

func cloneFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	destroyFlagName := "destroy"
	flags.BoolVar(&ctrClone.Destroy, destroyFlagName, false, "destroy the original container")

	runFlagName := "run"
	flags.BoolVar(&ctrClone.Run, runFlagName, false, "run the new container")

	forceFlagName := "force"
	flags.BoolVarP(&ctrClone.Force, forceFlagName, "f", false, "force the existing container to be destroyed")

	common.DefineCreateDefaults(&ctrClone.CreateOpts)
	common.DefineCreateFlags(cmd, &ctrClone.CreateOpts, entities.CloneMode)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerCloneCommand,
		Parent:  containerCmd,
	})

	cloneFlags(containerCloneCommand)
}

func clone(cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("must specify at least 1 argument: %w", define.ErrInvalidArg)
	case 2:
		ctrClone.CreateOpts.Name = args[1]
	case 3:
		ctrClone.CreateOpts.Name = args[1]
		ctrClone.Image = args[2]
		if !cliVals.RootFS {
			rawImageName := args[0]
			name, err := pullImage(cmd, ctrClone.Image, &ctrClone.CreateOpts)
			if err != nil {
				return err
			}
			ctrClone.Image = name
			ctrClone.RawImageName = rawImageName
		}
	}
	if ctrClone.Force && !ctrClone.Destroy {
		return fmt.Errorf("cannot set --force without --destroy: %w", define.ErrInvalidArg)
	}

	ctrClone.ID = args[0]
	ctrClone.CreateOpts.IsClone = true
	rep, err := registry.ContainerEngine().ContainerClone(registry.Context(), ctrClone)
	if err != nil {
		return err
	}
	fmt.Println(rep.Id)
	return nil
}
