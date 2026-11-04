package configs

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var rmCmd = &cobra.Command{
	Use:               "rm [options] CONFIG [CONFIG...]",
	Short:             "Remove one or more configs",
	RunE:              rm,
	ValidArgsFunction: common.AutocompleteConfigs,
	Example:           "podman config rm myconfig1 myconfig2",
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  configCmd,
	})
	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all configs")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified config is missing")
}

var rmOptions = entities.ConfigRmOptions{}

func rm(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if (len(args) > 0 && rmOptions.All) || (len(args) < 1 && !rmOptions.All) {
		return errors.New("`podman config rm` requires one argument, or the --all flag")
	}
	responses, err := registry.ContainerEngine().ConfigRm(cmd.Context(), args, rmOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.ID)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
