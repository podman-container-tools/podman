package images

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	importDescription = `Create a container image from the contents of the specified tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz, .tar.zst).

  Note remote tar balls can be specified, via web address.
  Optionally tag the image. You can specify the instructions using the --change option.`
	importCommand = &cobra.Command{
		Use:               "import [options] PATH [REFERENCE]",
		Short:             "Import a tarball to create a filesystem image",
		Long:              importDescription,
		RunE:              importCon,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman import https://example.com/ctr.tar url-image
cat ctr.tar | podman -q import --message "importing the ctr.tar tarball" - image-imported
cat ctr.tar | podman import -`,
	}

	imageImportCommand = &cobra.Command{
		Use:               importCommand.Use,
		Short:             importCommand.Short,
		Long:              importCommand.Long,
		RunE:              importCommand.RunE,
		Args:              importCommand.Args,
		ValidArgsFunction: importCommand.ValidArgsFunction,
		Example: `podman image import https://example.com/ctr.tar url-image
cat ctr.tar | podman -q image import --message "importing the ctr.tar tarball" - image-imported
cat ctr.tar | podman image import -`,
	}
)

var importOpts entities.ImageImportOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: importCommand,
	})
	importFlags(importCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageImportCommand,
		Parent:  imageCmd,
	})
	importFlags(imageImportCommand)
}

func importFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	changeFlagName := "change"
	flags.StringArrayVarP(&importOpts.Changes, changeFlagName, "c", []string{}, "Apply the following possible instructions to the created image (default []): "+strings.Join(common.ChangeCmds, " | "))
	_ = cmd.RegisterFlagCompletionFunc(changeFlagName, common.AutocompleteChangeInstructions)

	messageFlagName := "message"
	flags.StringVarP(&importOpts.Message, messageFlagName, "m", "", "Set commit message for imported image")
	_ = cmd.RegisterFlagCompletionFunc(messageFlagName, completion.AutocompleteNone)

	osFlagName := "os"
	flags.StringVar(&importOpts.OS, osFlagName, "", "Set the OS of the imported image")
	_ = cmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteNone)

	archFlagName := "arch"
	flags.StringVar(&importOpts.Architecture, archFlagName, "", "Set the architecture of the imported image")
	_ = cmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteNone)

	variantFlagName := "variant"
	flags.StringVar(&importOpts.Variant, variantFlagName, "", "Set the variant of the imported image")
	_ = cmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&importOpts.Quiet, "quiet", "q", false, "Suppress output")
	if !registry.IsRemote() {
		flags.StringVar(&importOpts.SignaturePolicy, "signature-policy", "", "Path to a signature-policy file")
		_ = flags.MarkHidden("signature-policy")
	}
}

func importCon(_ *cobra.Command, args []string) error {
	var (
		source    string
		reference string
	)
	switch len(args) {
	case 0:
		return errors.New("need to give the path to the tarball, or must specify a tarball of '-' for stdin")
	case 1:
		source = args[0]
	case 2:
		source = args[0]
		// TODO when save is merged, we need to process reference
		// like it is done in there or we end up with docker.io prepends
		// instead of the localhost ones
		reference = args[1]
	default:
		return errors.New("too many arguments. Usage TARBALL [REFERENCE]")
	}

	if source == "-" {
		outFile, err := os.CreateTemp("", "podman")
		if err != nil {
			return fmt.Errorf("creating file %v", err)
		}
		defer os.Remove(outFile.Name())
		defer outFile.Close()

		_, err = io.Copy(outFile, os.Stdin)
		if err != nil {
			return fmt.Errorf("copying file %v", err)
		}
		source = outFile.Name()
	}

	errFileName := parse.ValidateFileName(source)
	errURL := parse.ValidWebURL(source)
	if errURL == nil {
		importOpts.SourceIsURL = true
	}
	if errFileName != nil && errURL != nil {
		return multierror.Append(errFileName, errURL)
	}

	importOpts.Source = source
	importOpts.Reference = reference

	response, err := registry.ImageEngine().Import(context.Background(), importOpts)
	if err != nil {
		return err
	}
	fmt.Println(response.Id)
	return nil
}
