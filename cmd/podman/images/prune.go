package images

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	pruneDescription = `Removes dangling or unused images from local storage.`
	pruneCmd         = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove unused images",
		Long:              pruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman image prune`,
	}

	pruneOpts = entities.ImagePruneOptions{}
	force     bool
	filter    = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCmd,
		Parent:  buildxCmd,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCmd,
		Parent:  imageCmd,
	})

	flags := pruneCmd.Flags()
	flags.BoolVarP(&pruneOpts.All, "all", "a", false, "Remove all images not in use by containers, not just dangling ones")
	flags.BoolVarP(&pruneOpts.BuildCache, "build-cache", "", false, "Remove persistent build cache created by --mount=type=cache")
	flags.BoolVarP(&pruneOpts.External, "external", "", false, "Remove images even when they are used by external containers (e.g., by build containers)")
	flags.BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")

	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = pruneCmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePruneFilters)
}

func prune(_ *cobra.Command, _ []string) error {
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("%s", createPruneWarningMessage(pruneOpts))
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	filterMap, err := parse.FilterArgumentsIntoFilters(filter)
	if err != nil {
		return err
	}
	for k, v := range filterMap {
		for _, val := range v {
			pruneOpts.Filter = append(pruneOpts.Filter, fmt.Sprintf("%s=%s", k, val))
		}
	}
	results, err := registry.ImageEngine().Prune(registry.Context(), pruneOpts)
	if err != nil {
		return err
	}

	return utils.PrintImagePruneResults(results, false)
}

func createPruneWarningMessage(pruneOpts entities.ImagePruneOptions) string {
	question := "Are you sure you want to continue? [y/N] "
	if pruneOpts.All {
		return "WARNING! This command removes all images without at least one container associated with them.\n" + question
	}
	return "WARNING! This command removes all dangling images.\n" + question
}
