package artifact

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/buildah/pkg/cli"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// pullOptionsWrapper wraps entities.ImagePullOptions and prevents leaking
// CLI-only fields into the API types.
type pullOptionsWrapper struct {
	entities.ArtifactPullOptions
	TLSVerifyCLI   bool // CLI only
	CredentialsCLI string
	DecryptionKeys []string
}

var (
	pullOptions     = pullOptionsWrapper{}
	pullDescription = `Pulls an artifact from a registry and stores it locally.`

	pullCmd = &cobra.Command{
		Use:               "pull [options] ARTIFACT",
		Short:             "Pull an OCI artifact",
		Long:              pullDescription,
		RunE:              artifactPull,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example:           `podman artifact pull quay.io/myimage/myartifact:latest`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pullCmd,
		Parent:  artifactCmd,
	})
	pullFlags(pullCmd)
}

// pullFlags set the flags for the pull command.
func pullFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	credsFlagName := "creds"
	flags.StringVar(&pullOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&pullOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.BoolVar(&pullOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	authfileFlagName := "authfile"
	flags.StringVar(&pullOptions.AuthFilePath, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	decryptionKeysFlagName := "decryption-key"
	flags.StringArrayVar(&pullOptions.DecryptionKeys, decryptionKeysFlagName, nil, "Key needed to decrypt the image (e.g. /path/to/key.pem)")
	_ = cmd.RegisterFlagCompletionFunc(decryptionKeysFlagName, completion.AutocompleteDefault)

	retryFlagName := "retry"
	flags.Uint(retryFlagName, registry.RetryDefault(), "number of times to retry in case of failure when performing pull")
	_ = cmd.RegisterFlagCompletionFunc(retryFlagName, completion.AutocompleteNone)
	retryDelayFlagName := "retry-delay"
	flags.String(retryDelayFlagName, registry.RetryDelayDefault(), "delay between retries in case of pull failures")
	_ = cmd.RegisterFlagCompletionFunc(retryDelayFlagName, completion.AutocompleteNone)

	if registry.IsRemote() {
		_ = flags.MarkHidden(decryptionKeysFlagName)
	} else {
		certDirFlagName := "cert-dir"
		flags.StringVar(&pullOptions.CertDirPath, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
		_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)
	}
}

func artifactPull(cmd *cobra.Command, args []string) error {
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		pullOptions.InsecureSkipTLSVerify = types.NewOptionalBool(!pullOptions.TLSVerifyCLI)
	}

	if cmd.Flags().Changed("retry") {
		retry, err := cmd.Flags().GetUint("retry")
		if err != nil {
			return err
		}

		pullOptions.MaxRetries = &retry
	}

	if cmd.Flags().Changed("retry-delay") {
		val, err := cmd.Flags().GetString("retry-delay")
		if err != nil {
			return err
		}

		pullOptions.RetryDelay = val
	}

	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(pullOptions.AuthFilePath); err != nil {
			return err
		}
	}

	if pullOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(pullOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		pullOptions.Username = creds.Username
		pullOptions.Password = creds.Password
	}

	decConfig, err := cli.DecryptConfig(pullOptions.DecryptionKeys)
	if err != nil {
		return fmt.Errorf("unable to obtain decryption config: %w", err)
	}
	pullOptions.OciDecryptConfig = decConfig

	if !pullOptions.Quiet {
		pullOptions.Writer = os.Stdout
	}

	_, err = registry.ImageEngine().ArtifactPull(registry.Context(), args[0], pullOptions.ArtifactPullOptions)
	return err
}
