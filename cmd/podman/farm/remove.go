package farm

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	farmRmDescription = `Remove one or more existing farms.`
	rmCommand         = &cobra.Command{
		Use:                "remove [options] [FARM...]",
		Aliases:            []string{"rm"},
		Short:              "Remove one or more farms",
		Long:               farmRmDescription,
		PersistentPreRunE:  validate.NoOp,
		RunE:               rm,
		PersistentPostRunE: validate.NoOp,
		ValidArgsFunction:  common.AutoCompleteFarms,
		Example: `podman farm rm myfarm1 myfarm2
podman farm rm --all`,
	}

	// Temporary struct to hold cli values.
	rmOpts = struct {
		All bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  farmCmd,
	})
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOpts.All, "all", "a", false, "Remove all farms")
}

func rm(_ *cobra.Command, args []string) error {
	deletedFarms := []string{}
	err := config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if rmOpts.All {
			cfg.Farm.List = make(map[string][]string)
			cfg.Farm.Default = ""
			return nil
		}

		// If the --all is not set, we require at least one arg
		if len(args) == 0 {
			return errors.New("requires at lease 1 arg(s), received 0")
		}

		if len(cfg.Farm.List) == 0 {
			return errors.New("no existing farms; nothing to remove")
		}

		for _, k := range args {
			if _, ok := cfg.Farm.List[k]; !ok {
				logrus.Warnf("farm %q doesn't exist; nothing to remove", k)
				continue
			}
			delete(cfg.Farm.List, k)
			deletedFarms = append(deletedFarms, k)
			if k == cfg.Farm.Default {
				cfg.Farm.Default = ""
			}
		}
		// Return error if none of the given farms were deleted
		if len(deletedFarms) == 0 {
			return fmt.Errorf("failed to delete farms %q", args)
		}

		// Set a new default farm if the current default farm has been removed
		if cfg.Farm.Default == "" && cfg.Farm.List != nil {
			for k := range cfg.Farm.List {
				cfg.Farm.Default = k
				break
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if rmOpts.All {
		fmt.Println("All farms have been deleted")
		return nil
	}

	for _, k := range deletedFarms {
		fmt.Printf("Farm %q deleted\n", k)
	}
	return nil
}
