package images

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"golang.org/x/term"
)

var containerConfig = registry.PodmanConfig()

var (
	saveDescription = `Save an image to docker-archive or oci-archive on the local machine. Default is docker-archive.`

	saveCommand = &cobra.Command{
		Use:   "save [options] IMAGE [IMAGE...]",
		Short: "Save image(s) to an archive",
		Long:  saveDescription,
		RunE:  save,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("need at least 1 argument")
			}
			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}
			if !slices.Contains(common.ValidSaveFormats, format) {
				return fmt.Errorf("format value must be one of %s", strings.Join(common.ValidSaveFormats, " "))
			}
			return nil
		},
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman save --quiet -o myimage.tar imageID
podman save --format docker-dir -o ubuntu-dir ubuntu
podman save > alpine-all.tar alpine:latest`,
	}

	imageSaveCommand = &cobra.Command{
		Args:              saveCommand.Args,
		Use:               saveCommand.Use,
		Short:             saveCommand.Short,
		Long:              saveCommand.Long,
		RunE:              saveCommand.RunE,
		ValidArgsFunction: saveCommand.ValidArgsFunction,
		Example: `podman image save --quiet -o myimage.tar imageID
podman image save --format docker-dir -o ubuntu-dir ubuntu
podman image save > alpine-all.tar alpine:latest`,
	}
)

var saveOpts entities.ImageSaveOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: saveCommand,
	})
	saveFlags(saveCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageSaveCommand,
		Parent:  imageCmd,
	})
	saveFlags(imageSaveCommand)
}

func saveFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVar(&saveOpts.Compress, "compress", false, "Compress tarball image layers when saving to a directory using the 'dir' transport. (default is same compression type as source)")

	flags.BoolVar(&saveOpts.OciAcceptUncompressedLayers, "uncompressed", false, "Accept uncompressed layers when copying OCI images")

	formatFlagName := "format"
	flags.StringVar(&saveOpts.Format, formatFlagName, define.V2s2Archive, "Save image to oci-archive, oci-dir (directory with oci manifest type), docker-archive, docker-dir (directory with v2s2 manifest type)")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteImageSaveFormat)

	outputFlagName := "output"
	flags.StringVarP(&saveOpts.Output, outputFlagName, "o", "", "Write to a specified file (default: stdout, which must be redirected)")
	_ = cmd.RegisterFlagCompletionFunc(outputFlagName, completion.AutocompleteDefault)

	flags.BoolVarP(&saveOpts.Quiet, "quiet", "q", false, "Suppress the output")
	flags.BoolVarP(&saveOpts.MultiImageArchive, "multi-image-archive", "m", containerConfig.ContainersConfDefaultsRO.Engine.MultiImageArchive, "Interpret additional arguments as images not tags and create a multi-image-archive (only for docker-archive)")

	if !registry.IsRemote() {
		flags.StringVar(&saveOpts.SignaturePolicy, "signature-policy", "", "Path to a signature-policy file")
		_ = flags.MarkHidden("signature-policy")
	}
}

func save(cmd *cobra.Command, args []string) (finalErr error) {
	var (
		tags      []string
		succeeded = false
	)
	if cmd.Flag("compress").Changed && saveOpts.Format != define.V2s2ManifestDir {
		return errors.New("--compress can only be set when --format is 'docker-dir'")
	}
	if len(saveOpts.Output) == 0 {
		saveOpts.Quiet = true
		fi := os.Stdout
		if term.IsTerminal(int(fi.Fd())) {
			return errors.New("refusing to save to terminal. Use -o flag or redirect")
		}
		pipePath, cleanup, err := setupPipe()
		if err != nil {
			return err
		}
		if cleanup != nil {
			defer func() {
				errc := cleanup()
				if succeeded {
					writeErr := <-errc
					if writeErr != nil && finalErr == nil {
						finalErr = writeErr
					}
				}
			}()
		}
		saveOpts.Output = pipePath
	}
	if err := parse.ValidateFileName(saveOpts.Output); err != nil {
		return err
	}
	if len(args) > 1 {
		tags = args[1:]
	}

	err := registry.ImageEngine().Save(context.Background(), args[0], tags, saveOpts)
	if err == nil {
		succeeded = true
	}
	return err
}
