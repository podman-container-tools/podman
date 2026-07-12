package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

var (
	rmOptions = entities.ImageRemoveOptions{}
	rmCmd     = &cobra.Command{
		Use:               "rm [options] LIST [LIST...]",
		Short:             "Remove manifest list or image index from local storage",
		Long:              "Remove manifest list or image index from local storage.",
		RunE:              rm,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteImages,
		Example:           `podman manifest rm mylist:v1.11`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  manifestCmd,
	})

	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified manifest is missing")
}

func rm(_ *cobra.Command, args []string) error {
	report, rmErrors := registry.ImageEngine().ManifestRm(context.Background(), args, rmOptions)
	if report != nil {
		for _, u := range report.Untagged {
			fmt.Println("Untagged: " + u)
		}
		for _, d := range report.Deleted {
			// Make sure an image was deleted (and not just untagged); else print it
			if len(d) > 0 {
				fmt.Println("Deleted: " + d)
			}
		}
		registry.SetExitCode(report.ExitCode)
	}

	return errorhandling.JoinErrors(rmErrors)
}
