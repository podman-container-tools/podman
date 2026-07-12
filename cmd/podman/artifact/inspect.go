package artifact

import (
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] ARTIFACT",
		Short:             "Inspect an OCI artifact",
		Long:              "Provide details on an OCI artifact",
		RunE:              artifactInspect,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example:           `podman artifact inspect quay.io/myimage/myartifact:latest`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  artifactCmd,
	})

	inspectOpts = new(entities.InspectOptions)

	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format volume output using JSON or a Go template")

	// This is something we wanted to do but did not seem important enough for initial PR
	// remoteFlagName := "remote"
	// flags.BoolVar(&inspectFlag.remote, remoteFlagName, false, "Inspect the image on a container image registry")

	// TODO When the inspect structure has been defined, we need to uncomment and redirect this.  Reminder, this
	// will also need to be reflected in the podman-artifact-inspect man page
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ArtifactInspectReport{}))
}

func artifactInspect(_ *cobra.Command, args []string) error {
	artifactOptions := entities.ArtifactInspectOptions{}
	inspectData, err := registry.ImageEngine().ArtifactInspect(registry.Context(), args[0], artifactOptions)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(inspectOpts.Format) || inspectOpts.Format == "":
		return utils.PrintGenericJSON(inspectData)
	default:
		// Landing here implies user has given a custom --format
		var rpt *report.Formatter
		format := inspect.InspectNormalize(inspectOpts.Format, inspectOpts.Type)
		rpt, err = report.New(os.Stdout, "inspect").Parse(report.OriginUser, format)
		if err != nil {
			return err
		}
		defer rpt.Flush()

		// Storing and passing inspectData in an array to [Execute] is workaround to avoid getting an error.
		// Which seems to happen when type passed to [Execute] is not a slice.
		// Error: template: inspect:1:8: executing "inspect" at <.>: range can't iterate over {0x6600020c444 sha256:4bafff5c1b2c950651101d22d3dbf76744446aeb5f79fc926674e0db1083qew456}
		data := []any{inspectData}
		return rpt.Execute(data)
	}
}
