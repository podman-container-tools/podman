package pods

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	podCloneDescription = `Create an exact copy of a pod and the containers within it`

	podCloneCommand = &cobra.Command{
		Use:               "clone [options] POD NAME",
		Short:             "Clone an existing pod",
		Long:              podCloneDescription,
		RunE:              clone,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteClone,
		Example:           `podman pod clone pod_name new_name`,
	}
)

var podClone entities.PodCloneOptions

func cloneFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	destroyFlagName := "destroy"
	flags.BoolVar(&podClone.Destroy, destroyFlagName, false, "destroy the original pod")

	startFlagName := "start"
	flags.BoolVar(&podClone.Start, startFlagName, false, "start the new pod")

	nameFlagName := "name"
	flags.StringVarP(&podClone.CreateOpts.Name, nameFlagName, "n", "", "name the new pod")
	_ = podCloneCommand.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)

	common.DefineCreateDefaults(&podClone.InfraOptions)
	common.DefineCreateFlags(cmd, &podClone.InfraOptions, entities.InfraMode)

	podClone.InfraOptions.MemorySwappiness = -1 // this is not implemented for pods yet, need to set -1 default manually

	// need to fill an empty ctr create option for each container for sane defaults
	// for now, these cannot be used. The flag names conflict too much
	// this makes sense since this is a pod command not a container command
	// TODO: add support for container specific arguments/flags
	common.DefineCreateDefaults(&podClone.PerContainerOptions)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: podCloneCommand,
		Parent:  podCmd,
	})

	cloneFlags(podCloneCommand)
}

func clone(cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("must specify at least 1 argument: %w", define.ErrInvalidArg)
	case 2:
		podClone.CreateOpts.Name = args[1]
	}

	podClone.ID = args[0]

	if cmd.Flag("shm-size").Changed {
		podClone.InfraOptions.ShmSize = cmd.Flag("shm-size").Value.String()
	}

	podClone.PerContainerOptions.IsClone = true
	rep, err := registry.ContainerEngine().PodClone(context.Background(), podClone)
	if err != nil {
		if rep != nil {
			fmt.Printf("pod %s created but error after creation\n", rep.Id)
		}
		return err
	}

	fmt.Println(rep.Id)

	return nil
}
