package generate

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var specCmd = &cobra.Command{
	Use:               "spec [options] {CONTAINER|POD}",
	Short:             "Generate Specgen JSON based on containers or pods",
	Long:              "Generate Specgen JSON based on containers or pods",
	RunE:              spec,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: common.AutocompleteContainersAndPods,
	Example:           `podman generate spec ctrID`,
}

var opts *entities.GenerateSpecOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: specCmd,
		Parent:  GenerateCmd,
	})
	opts = &entities.GenerateSpecOptions{}
	flags := specCmd.Flags()

	filenameFlagName := "filename"
	flags.StringVarP(&opts.FileName, filenameFlagName, "f", "", "Write output to the specified path")
	_ = specCmd.RegisterFlagCompletionFunc(filenameFlagName, completion.AutocompleteNone)

	compactFlagName := "compact"
	flags.BoolVarP(&opts.Compact, compactFlagName, "c", false, "Print the json in a compact format for consumption")

	nameFlagName := "name"
	flags.BoolVarP(&opts.Name, nameFlagName, "n", true, "Specify a new name for the generated spec")

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func spec(_ *cobra.Command, args []string) error {
	opts.ID = args[0]
	report, err := registry.ContainerEngine().GenerateSpec(registry.Context(), opts)
	if err != nil {
		return err
	}

	// if we are looking to print the output, do not mess it up by printing the path
	// if we are using -v the user probably expects to pipe the output somewhere else
	if len(opts.FileName) > 0 {
		err = os.WriteFile(opts.FileName, report.Data, 0o644)
		if err != nil {
			return err
		}
		fmt.Println(opts.FileName)
	} else {
		fmt.Println(string(report.Data))
	}
	return nil
}
