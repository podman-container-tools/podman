package network

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	networkPruneDescription = `Prune unused networks`
	networkPruneCommand     = &cobra.Command{
		Use:               "prune [options]",
		Short:             "Prune unused networks",
		Long:              networkPruneDescription,
		RunE:              networkPrune,
		Example:           `podman network prune`,
		Args:              validate.NoArgs,
		ValidArgsFunction: common.AutocompleteNetworks,
	}
)

var (
	networkPruneOptions entities.NetworkPruneOptions
	force               bool
	filter              = []string{}
)

func networkPruneFlags(cmd *cobra.Command, flags *pflag.FlagSet) {
	flags.BoolVarP(&force, "force", "f", false, "do not prompt for confirmation")
	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePruneFilters)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkPruneCommand,
		Parent:  networkCmd,
	})
	flags := networkPruneCommand.Flags()
	networkPruneFlags(networkPruneCommand, flags)
}

func networkPrune(_ *cobra.Command, _ []string) error {
	var err error
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all networks not used by at least one container.")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	networkPruneOptions.Filters, err = parse.FilterArgumentsIntoFilters(filter)
	if err != nil {
		return err
	}
	responses, err := registry.ContainerEngine().NetworkPrune(registry.Context(), networkPruneOptions)
	if err != nil {
		setExitCode(err)
		return err
	}
	return utils.PrintNetworkPruneResults(responses, false)
}
