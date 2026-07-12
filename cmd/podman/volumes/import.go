package volumes

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	importDescription = `Imports contents into a podman volume from specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz, .tar.zst).`
	importCommand     = &cobra.Command{
		Use:               "import VOLUME [SOURCE]",
		Short:             "Import a tarball contents into a podman volume",
		Long:              importDescription,
		RunE:              importVol,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume import my_vol /home/user/import.tar
cat ctr.tar | podman volume import my_vol -`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: importCommand,
		Parent:  volumeCmd,
	})
}

func importVol(_ *cobra.Command, args []string) error {
	opts := entities.VolumeImportOptions{}

	filepath := args[1]
	if filepath == "-" {
		opts.Input = os.Stdin
	} else {
		if err := parse.ValidateFileName(filepath); err != nil {
			return err
		}

		targetFile, err := os.Open(filepath)
		if err != nil {
			return fmt.Errorf("unable open input file: %w", err)
		}
		defer targetFile.Close()
		opts.Input = targetFile
	}

	containerEngine := registry.ContainerEngine()
	ctx := context.Background()

	return containerEngine.VolumeImport(ctx, args[0], opts)
}
