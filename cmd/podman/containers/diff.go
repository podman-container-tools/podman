package containers

import (
	"errors"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/diff"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	// podman container _diff_
	diffCmd = &cobra.Command{
		Use:               "diff [options] CONTAINER [CONTAINER]",
		Args:              diff.ValidateContainerDiffArgs,
		Short:             "Inspect changes to the container's file systems",
		Long:              `Displays changes to the container filesystem's'.  The container will be compared to its parent layer or the second argument when given.`,
		RunE:              diffRun,
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman container diff myCtr
podman container diff -l --format json myCtr`,
	}
	diffOpts *entities.DiffOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: diffCmd,
		Parent:  containerCmd,
	})

	diffOpts = new(entities.DiffOptions)
	flags := diffCmd.Flags()

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format (json)")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	validate.AddLatestFlag(diffCmd, &diffOpts.Latest)
}

func diffRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !diffOpts.Latest {
		return errors.New("container must be specified: podman container diff [options [...]] ID-NAME")
	}
	diffOpts.Type = define.DiffContainer
	return diff.Diff(cmd, args, *diffOpts)
}
