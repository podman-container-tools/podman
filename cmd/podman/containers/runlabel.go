package containers

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// runlabelOptionsWrapper allows for combining API-only with CLI-only options
// and to convert between them.
type runlabelOptionsWrapper struct {
	entities.ContainerRunlabelOptions
	TLSVerifyCLI bool
}

var (
	runlabelOptions     = runlabelOptionsWrapper{}
	runlabelDescription = "Executes a command as described by a container image label."
	runlabelCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "runlabel [options] LABEL IMAGE [ARG...]",
		Short:             "Execute the command described by an image label",
		Long:              runlabelDescription,
		RunE:              runlabel,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteRunlabelCommand,
		Example: `podman container runlabel run imageID
podman container runlabel install imageID arg1 arg2
podman container runlabel --display run myImage`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: runlabelCommand,
		Parent:  containerCmd,
	})

	flags := runlabelCommand.Flags()

	authfileflagName := "authfile"
	flags.StringVar(&runlabelOptions.Authfile, authfileflagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = runlabelCommand.RegisterFlagCompletionFunc(authfileflagName, completion.AutocompleteDefault)

	certDirFlagName := "cert-dir"
	flags.StringVar(&runlabelOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
	_ = runlabelCommand.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

	credsFlagName := "creds"
	flags.StringVar(&runlabelOptions.Credentials, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = runlabelCommand.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	flags.BoolVar(&runlabelOptions.Display, "display", false, "Preview the command that the label would run")

	nameFlagName := "name"
	flags.StringVarP(&runlabelOptions.Name, nameFlagName, "n", "", "Assign a name to the container")
	_ = runlabelCommand.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)

	flags.StringVar(&runlabelOptions.Optional1, "opt1", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelOptions.Optional2, "opt2", "", "Optional parameter to pass for install")
	flags.StringVar(&runlabelOptions.Optional3, "opt3", "", "Optional parameter to pass for install")
	flags.BoolVarP(&runlabelOptions.Pull, "pull", "p", true, "Pull the image if it does not exist locally prior to executing the label contents")
	flags.BoolVarP(&runlabelOptions.Quiet, "quiet", "q", false, "Suppress output information when installing images")
	flags.BoolVar(&runlabelOptions.Replace, "replace", false, "Replace existing container with a new one from the image")
	flags.BoolVar(&runlabelOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	// Hide the optional flags.
	_ = flags.MarkHidden("opt1")
	_ = flags.MarkHidden("opt2")
	_ = flags.MarkHidden("opt3")
	_ = flags.MarkHidden("pull")
	if !registry.IsRemote() {
		flags.StringVar(&runlabelOptions.SignaturePolicy, "signature-policy", "", "`Pathname` of signature policy file (not usually used)")
		_ = flags.MarkHidden("signature-policy")
	}
	if err := flags.MarkDeprecated("pull", "podman will pull if not found in local storage"); err != nil {
		logrus.Error("unable to mark pull flag deprecated")
	}
}

func runlabel(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("tls-verify") {
		runlabelOptions.SkipTLSVerify = types.NewOptionalBool(!runlabelOptions.TLSVerifyCLI)
	}
	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(runlabelOptions.Authfile); err != nil {
			return err
		}
	}
	return registry.ContainerEngine().ContainerRunlabel(context.Background(), strings.TrimPrefix(args[0], "/"), args[1], args[2:], runlabelOptions.ContainerRunlabelOptions)
}
