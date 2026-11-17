package images

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/diff"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	// podman container _inspect_
	diffCmd = &cobra.Command{
		Use:               "diff [options] IMAGE [IMAGE]",
		Args:              cobra.RangeArgs(1, 2),
		Short:             "Inspect changes to the image's file systems",
		Long:              `Displays changes to the image's filesystem.  The image will be compared to its parent layer or the second argument when given.`,
		RunE:              diffRun,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image diff myImage
podman image diff --format json redis:alpine`,
	}
	diffOpts *entities.DiffOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: diffCmd,
		Parent:  imageCmd,
	})
	diffFlags(diffCmd.Flags())
}

func diffFlags(flags *pflag.FlagSet) {
	diffOpts = new(entities.DiffOptions)

	formatFlagName := "format"
	flags.StringVar(&diffOpts.Format, formatFlagName, "", "Change the output format (json)")
	_ = diffCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))
}

func diffRun(cmd *cobra.Command, args []string) error {
	diffOpts.Type = define.DiffImage
	return diff.Diff(cmd, args, *diffOpts)
}
