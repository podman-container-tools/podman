package pods

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var pruneOptions = entities.PodPruneOptions{}

var (
	pruneDescription = `podman pod prune Removes all exited pods`

	pruneCommand = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove all stopped pods and their containers",
		Long:              pruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman pod prune`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCommand,
		Parent:  podCmd,
	})
	flags := pruneCommand.Flags()
	flags.BoolVarP(&pruneOptions.Force, "force", "f", false, "Do not prompt for confirmation.  The default is false")
}

func prune(_ *cobra.Command, _ []string) error {
	if !pruneOptions.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("WARNING! This will remove all stopped/exited pods..")
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	responses, err := registry.ContainerEngine().PodPrune(context.Background(), pruneOptions)
	if err != nil {
		return err
	}
	return utils.PrintPodPruneResults(responses, false)
}
