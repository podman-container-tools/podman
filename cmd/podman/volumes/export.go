package volumes

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"golang.org/x/term"
)

var (
	volumeExportDescription = `
podman volume export

Allow content of volume to be exported into external tar.`
	exportCommand = &cobra.Command{
		Use:               "export [options] VOLUME",
		Short:             "Export volumes",
		Args:              cobra.ExactArgs(1),
		Long:              volumeExportDescription,
		RunE:              export,
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

var targetPath string

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: exportCommand,
		Parent:  volumeCmd,
	})
	flags := exportCommand.Flags()

	outputFlagName := "output"
	flags.StringVarP(&targetPath, outputFlagName, "o", "", "Write to a specified file (default: stdout, which must be redirected)")
	_ = exportCommand.RegisterFlagCompletionFunc(outputFlagName, completion.AutocompleteDefault)
}

func export(cmd *cobra.Command, args []string) error {
	containerEngine := registry.ContainerEngine()
	ctx := context.Background()
	exportOpts := entities.VolumeExportOptions{}

	if targetPath != "" {
		targetFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("unable to create target file path %q: %w", targetPath, err)
		}
		defer targetFile.Close()
		exportOpts.Output = targetFile
	} else {
		if cmd.Flag("output").Changed {
			return errors.New("must provide valid path for file to write to")
		}
		if term.IsTerminal(int(os.Stdout.Fd())) {
			return errors.New("cannot write to terminal, use command-line redirection or the --output flag")
		}
		exportOpts.Output = os.Stdout
	}

	return containerEngine.VolumeExport(ctx, args[0], exportOpts)
}
