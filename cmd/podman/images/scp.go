package images

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/utils"
)

var (
	saveScpDescription = `Securely copy an image from one host to another.`
	imageScpCommand    = &cobra.Command{
		Use: "scp [options] IMAGE [HOST::]",
		Annotations: map[string]string{
			registry.ParentNSRequired: "",
		},
		Long:              saveScpDescription,
		Short:             "Securely copy images",
		RunE:              scp,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteScp,
		Example:           `podman image scp myimage:latest otherhost::`,
	}
)

var (
	parentFlags       []string
	quiet             bool
	format            string
	scpCompressFormat string
	scpCompressLevel  int
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageScpCommand,
		Parent:  imageCmd,
	})
	scpFlags(imageScpCommand)
}

func scpFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVarP(&quiet, "quiet", "q", false, "Suppress the output")

	formatChoice := validate.Value(&format, common.ValidScpFormats...)
	flags.Var(formatChoice, "format", "Format for `podman save` when creating the transfer archive ("+formatChoice.Choices()+"). Default is docker-archive when omitted.")
	_ = cmd.RegisterFlagCompletionFunc("format", common.AutocompleteImageScpFormat)

	compFormatFlagName := "compression-format"
	compFormatChoice := validate.Value(&scpCompressFormat, utils.ScpCompressionFormats()...)
	flags.Var(compFormatChoice, compFormatFlagName, "Compress the transfer archive with the given algorithm ("+compFormatChoice.Choices()+"). Default is no compression.")
	_ = cmd.RegisterFlagCompletionFunc(compFormatFlagName, common.AutocompleteImageScpCompressionFormat)

	compLevelFlagName := "compression-level"
	flags.IntVar(&scpCompressLevel, compLevelFlagName, 0, "Compression level to use")
	_ = cmd.RegisterFlagCompletionFunc(compLevelFlagName, completion.AutocompleteNone)
}

func scp(cmd *cobra.Command, args []string) (finalErr error) {
	var err error

	compressOpts := entities.ScpCompressionOptions{CompressionFormat: scpCompressFormat}
	if cmd.Flags().Changed("compression-level") {
		compressOpts.CompressionLevel = &scpCompressLevel
	}
	// Report a bad combination before anything expensive happens. The transfer
	// validates again for the sake of callers coming in over the API.
	if err := utils.ValidateScpCompression(compressOpts); err != nil {
		return err
	}

	containerConfig := registry.PodmanConfig()

	sshType := containerConfig.SSHMode

	for i, val := range os.Args {
		if val == "image" {
			break
		}
		if i == 0 {
			continue
		}
		if strings.Contains(val, "CIRRUS") { // need to skip CIRRUS flags for testing suite purposes
			continue
		}
		parentFlags = append(parentFlags, val)
	}

	src := args[0]
	dst := ""
	if len(args) > 1 {
		dst = args[1]
	}

	sshEngine := ssh.DefineMode(sshType)
	scpOpts := entities.ImageScpOptions{}
	scpOpts.ParentFlags = parentFlags
	scpOpts.Quiet = quiet
	scpOpts.SSHMode = sshEngine
	scpOpts.SaveFormat = format
	scpOpts.ScpCompressionOptions = compressOpts
	_, err = registry.ImageEngine().Scp(registry.Context(), src, dst, scpOpts)
	if err != nil {
		return err
	}

	return nil
}
