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

// pushOptionsWrapper wraps entities.ImagepushOptions and prevents leaking
// CLI-only fields into the API types.
type pushOptionsWrapper struct {
	entities.ArtifactPushOptions
	TLSVerifyCLI   bool // CLI only
	CredentialsCLI string
	signing        common.SigningCLIOnlyOptions
	EncryptionKeys []string
	EncryptLayers  []int
	DigestFile     string
}

var (
	pushOptions     = pushOptionsWrapper{}
	pushDescription = `Push an OCI artifact from local storage to an image registry`

	pushCmd = &cobra.Command{
		Use:               "push [options] ARTIFACT.",
		Short:             "Push an OCI artifact",
		Long:              pushDescription,
		RunE:              artifactPush,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example:           `podman artifact push quay.io/myimage/myartifact:latest`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pushCmd,
		Parent:  artifactCmd,
	})
	pushFlags(pushCmd)
}

// pushFlags sets the flags for the push command.
func pushFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	// For now default All flag to true, for pushing of manifest lists
	pushOptions.All = true
	authfileFlagName := "authfile"
	flags.StringVar(&pushOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	certDirFlagName := "cert-dir"
	flags.StringVar(&pushOptions.CertDir, certDirFlagName, "", "Path to a directory containing TLS certificates and keys")
	_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

	// This is a flag I didn't wire up but could be considered
	// flags.BoolVar(&pushOptions.Compress, "compress", false, "Compress tarball image layers when pushing to a directory using the 'dir' transport. (default is same compression type as source)")

	credsFlagName := "creds"
	flags.StringVar(&pushOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	digestfileFlagName := "digestfile"
	flags.StringVar(&pushOptions.DigestFile, digestfileFlagName, "", "Write the digest of the pushed image to the specified file")
	_ = cmd.RegisterFlagCompletionFunc(digestfileFlagName, completion.AutocompleteDefault)

	flags.BoolVarP(&pushOptions.Quiet, "quiet", "q", false, "Suppress output information when pushing images")

	retryFlagName := "retry"
	flags.Uint(retryFlagName, registry.RetryDefault(), "number of times to retry in case of failure when performing push")
	_ = cmd.RegisterFlagCompletionFunc(retryFlagName, completion.AutocompleteNone)

	retryDelayFlagName := "retry-delay"
	flags.String(retryDelayFlagName, registry.RetryDelayDefault(), "delay between retries in case of push failures")
	_ = cmd.RegisterFlagCompletionFunc(retryDelayFlagName, completion.AutocompleteNone)

	common.DefineSigningFlags(cmd, &pushOptions.signing, &pushOptions.ArtifactPushOptions.ImagePushOptions)

	flags.BoolVar(&pushOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	// TODO I think these two can be removed?
	/*
		compFormat := "compression-format"
		flags.StringVar(&pushOptions.CompressionFormat, compFormat, compressionFormat(), "compression format to use")
		_ = cmd.RegisterFlagCompletionFunc(compFormat, common.AutocompleteCompressionFormat)

		compLevel := "compression-level"
		flags.Int(compLevel, compressionLevel(), "compression level to use")
		_ = cmd.RegisterFlagCompletionFunc(compLevel, completion.AutocompleteNone)

	*/

	// Potential options that could be wired up if deemed necessary
	// encryptionKeysFlagName := "encryption-key"
	// flags.StringArrayVar(&pushOptions.EncryptionKeys, encryptionKeysFlagName, nil, "Key with the encryption protocol to use to encrypt the image (e.g. jwe:/path/to/key.pem)")
	// _ = cmd.RegisterFlagCompletionFunc(encryptionKeysFlagName, completion.AutocompleteDefault)

	// encryptLayersFlagName := "encrypt-layer"
	// flags.IntSliceVar(&pushOptions.EncryptLayers, encryptLayersFlagName, nil, "Layers to encrypt, 0-indexed layer indices with support for negative indexing (e.g. 0 is the first layer, -1 is the last layer). If not defined, will encrypt all layers if encryption-key flag is specified")
	// _ = cmd.RegisterFlagCompletionFunc(encryptLayersFlagName, completion.AutocompleteDefault)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
		_ = flags.MarkHidden("compress")
		_ = flags.MarkHidden("quiet")
	} else {
		signaturePolicyFlagName := "signature-policy"
		flags.StringVar(&pushOptions.SignaturePolicy, signaturePolicyFlagName, "", "Path to a signature-policy file")
		_ = flags.MarkHidden(signaturePolicyFlagName)
	}
}

func artifactPush(cmd *cobra.Command, args []string) error {
	source := args[0]
	// Should we just make destination == origin ?
	// destination := args[len(args)-1]

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		pushOptions.SkipTLSVerify = types.NewOptionalBool(!pushOptions.TLSVerifyCLI)
	}

	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(pushOptions.Authfile); err != nil {
			return err
		}
	}

	if pushOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(pushOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		pushOptions.Username = creds.Username
		pushOptions.Password = creds.Password
	}

	if !pushOptions.Quiet {
		pushOptions.Writer = os.Stderr
	}

	signingCleanup, err := common.PrepareSigning(&pushOptions.ImagePushOptions, &pushOptions.signing)
	if err != nil {
		return err
	}
	defer signingCleanup()

	encConfig, encLayers, err := cli.EncryptConfig(pushOptions.EncryptionKeys, pushOptions.EncryptLayers)
	if err != nil {
		return fmt.Errorf("unable to obtain encryption config: %w", err)
	}
	pushOptions.OciEncryptConfig = encConfig
	pushOptions.OciEncryptLayers = encLayers

	if cmd.Flags().Changed("retry") {
		retry, err := cmd.Flags().GetUint("retry")
		if err != nil {
			return err
		}

		pushOptions.Retry = &retry
	}

	if cmd.Flags().Changed("retry-delay") {
		val, err := cmd.Flags().GetString("retry-delay")
		if err != nil {
			return err
		}

		pushOptions.RetryDelay = val
	}

	// TODO If not compression options are supported, we do not need the following
	/*
		if cmd.Flags().Changed("compression-level") {
			val, err := cmd.Flags().GetInt("compression-level")
			if err != nil {
				return err
			}
			pushOptions.CompressionLevel = &val
		}

			if cmd.Flags().Changed("compression-format") {
				if !cmd.Flags().Changed("force-compression") {
					// If `compression-format` is set and no value for `--force-compression`
					// is selected then defaults to `true`.
					pushOptions.ForceCompressionFormat = true
				}
			}
	*/

	_, err = registry.ImageEngine().ArtifactPush(registry.Context(), source, pushOptions.ArtifactPushOptions)
	return err
}
