package images

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
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

	flags.BoolVar(&saveOpts.Zip, "zip", false, "Compress the output image using gzip")

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
	// If --zip is used, we need to manually create a writer that gzips the content.
	if saveOpts.Zip {
		// Determine the final output writer (stdout or a file)
		var out io.Writer = os.Stdout
		if len(saveOpts.Output) > 0 {
			f, err := os.Create(saveOpts.Output)
			if err != nil {
				return fmt.Errorf("could not create output file %s: %w", saveOpts.Output, err)
			}
			// Defer closing the file until the function returns
			defer f.Close()
			out = f
		} else {
			// Suppress progress output if writing to stdout
			saveOpts.Quiet = true
			if term.IsTerminal(int(os.Stdout.Fd())) {
				return errors.New("refusing to save to terminal. Use -o flag or redirect")
			}
		}

		// Create a temporary pipe. The image engine will write uncompressed data to the pipe's writer.
		r, w, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("could not create pipe: %w", err)
		}
		// Set the save operation's output to the pipe's writer end.
		saveOpts.Output = w.Name()

		// Create a gzip writer that writes to our final destination (file or stdout)
		gzw := gzip.NewWriter(out)

		// This goroutine will read from the pipe, compress the data, and write to the final destination.
		done := make(chan error)
		go func() {
			defer r.Close()
			defer gzw.Close()
			_, err := io.Copy(gzw, r)
			done <- err
		}()

		// Now, call the image engine. It will write the uncompressed tar to the pipe.
		saveErr := registry.ImageEngine().Save(context.Background(), args[0], args[1:], saveOpts)

		// Close the writer end of the pipe to signal EOF to the reader goroutine.
		w.Close()

		// Wait for the compression goroutine to finish and check for errors.
		compressErr := <-done

		if saveErr != nil {
			return saveErr
		}
		return compressErr
	}

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
