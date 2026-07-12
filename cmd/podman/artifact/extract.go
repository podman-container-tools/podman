package artifact

import (
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var extractCmd = &cobra.Command{
	Use:               "extract [options] ARTIFACT PATH",
	Short:             "Extract an OCI artifact to a local path",
	Long:              "Extract the blobs of an OCI artifact to a local file or directory",
	RunE:              extract,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: common.AutocompleteArtifactAdd,
	Example: `podman artifact Extract quay.io/myimage/myartifact:latest /tmp/foobar.txt
podman artifact Extract quay.io/myimage/myartifact:latest /home/paul/mydir`,
}

var extractOpts entities.ArtifactExtractOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: extractCmd,
		Parent:  artifactCmd,
	})
	flags := extractCmd.Flags()

	digestFlagName := "digest"
	flags.StringVar(&extractOpts.Digest, digestFlagName, "", "Only extract blob with the given digest")
	_ = extractCmd.RegisterFlagCompletionFunc(digestFlagName, completion.AutocompleteNone)

	titleFlagName := "title"
	flags.StringVar(&extractOpts.Title, titleFlagName, "", "Only extract blob with the given title")
	_ = extractCmd.RegisterFlagCompletionFunc(titleFlagName, completion.AutocompleteNone)
}

func extract(_ *cobra.Command, args []string) error {
	err := registry.ImageEngine().ArtifactExtract(registry.Context(), args[0], args[1], extractOpts)
	if err != nil {
		return err
	}

	return nil
}
