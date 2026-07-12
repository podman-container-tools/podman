package secrets

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var rmCmd = &cobra.Command{
	Use:               "rm [options] SECRET [SECRET...]",
	Short:             "Remove one or more secrets",
	RunE:              rm,
	ValidArgsFunction: common.AutocompleteSecrets,
	Example:           "podman secret rm mysecret1 mysecret2",
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  secretCmd,
	})
	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all secrets")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified secret is missing")
}

var rmOptions = entities.SecretRmOptions{}

func rm(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if (len(args) > 0 && rmOptions.All) || (len(args) < 1 && !rmOptions.All) {
		return errors.New("`podman secret rm` requires one argument, or the --all flag")
	}
	responses, err := registry.ContainerEngine().SecretRm(context.Background(), args, rmOptions)
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
