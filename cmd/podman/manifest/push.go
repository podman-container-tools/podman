package manifest

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// manifestPushOptsWrapper wraps entities.ManifestPushOptions and prevents leaking
// CLI-only fields into the API types.
type manifestPushOptsWrapper struct {
	entities.ImagePushOptions

	TLSVerifyCLI, Insecure bool // CLI only
	CredentialsCLI         string
	signing                common.SigningCLIOnlyOptions
	DigestFile             string
}

var (
	manifestPushOpts = manifestPushOptsWrapper{}
	pushCmd          = &cobra.Command{
		Use:               "push [options] LIST DESTINATION",
		Short:             "Push a manifest list or image index to a registry",
		Long:              "Pushes manifest lists and image indexes to registries.",
		RunE:              push,
		Example:           `podman manifest push mylist:v1.11 docker://quay.io/myuser/image:v1.11`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteImages,
	}
)

func init() {
	podmanConfig := registry.PodmanConfig()
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pushCmd,
		Parent:  manifestCmd,
	})
	flags := pushCmd.Flags()
	flags.BoolVar(&manifestPushOpts.Rm, "rm", false, "remove the manifest list if push succeeds")
	flags.BoolVar(&manifestPushOpts.All, "all", true, "also push the images in the list")
	flags.BoolVarP(&manifestPushOpts.Rm, "purge", "p", false, "remove the local manifest list after push")
	_ = flags.MarkHidden("purge")

	authfileFlagName := "authfile"
	flags.StringVar(&manifestPushOpts.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = pushCmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	addCompressionFlagName := "add-compression"
	flags.StringSliceVar(&manifestPushOpts.AddCompression, addCompressionFlagName, podmanConfig.ContainersConfDefaultsRO.Engine.AddCompression.Get(), "add instances with selected compression while pushing")
	_ = pushCmd.RegisterFlagCompletionFunc(addCompressionFlagName, common.AutocompleteCompressionFormat)

	certDirFlagName := "cert-dir"
	flags.StringVar(&manifestPushOpts.CertDir, certDirFlagName, "", "use certificates at the specified path to access the registry")
	_ = pushCmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

	credsFlagName := "creds"
	flags.StringVar(&manifestPushOpts.CredentialsCLI, credsFlagName, "", "use `[username[:password]]` for accessing the registry")
	_ = pushCmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	digestfileFlagName := "digestfile"
	flags.StringVar(&manifestPushOpts.DigestFile, digestfileFlagName, "", "after copying the image, write the digest of the resulting digest to the file")
	_ = pushCmd.RegisterFlagCompletionFunc(digestfileFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&manifestPushOpts.ForceCompressionFormat, "force-compression", false, "Use the specified compression algorithm even if the destination contains a differently-compressed variant already")

	formatFlagName := "format"
	flags.StringVarP(&manifestPushOpts.Format, formatFlagName, "f", "", "manifest type (oci or v2s2) to attempt to use when pushing the manifest list (default is manifest type of source)")
	_ = pushCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteManifestFormat)

	flags.BoolVarP(&manifestPushOpts.RemoveSignatures, "remove-signatures", "", false, "don't copy signatures when pushing images")

	common.DefineSigningFlags(pushCmd, &manifestPushOpts.signing, &manifestPushOpts.ImagePushOptions)

	flags.BoolVar(&manifestPushOpts.TLSVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	flags.BoolVar(&manifestPushOpts.Insecure, "insecure", false, "neither require HTTPS nor verify certificates when accessing the registry")
	_ = flags.MarkHidden("insecure")
	flags.BoolVarP(&manifestPushOpts.Quiet, "quiet", "q", false, "don't output progress information when pushing lists")
	flags.SetNormalizeFunc(utils.AliasFlags)

	retryFlagName := "retry"
	flags.Uint(retryFlagName, registry.RetryDefault(), "number of times to retry in case of failure when performing push")
	_ = pushCmd.RegisterFlagCompletionFunc(retryFlagName, completion.AutocompleteNone)
	retryDelayFlagName := "retry-delay"
	flags.String(retryDelayFlagName, registry.RetryDelayDefault(), "delay between retries in case of push failures")
	_ = pushCmd.RegisterFlagCompletionFunc(retryDelayFlagName, completion.AutocompleteNone)

	compressionFormat := "compression-format"
	flags.StringVar(&manifestPushOpts.CompressionFormat, compressionFormat, "", "compression format to use")
	_ = pushCmd.RegisterFlagCompletionFunc(compressionFormat, common.AutocompleteCompressionFormat)

	compressionLevel := "compression-level"
	flags.Int(compressionLevel, 0, "compression level to use")
	_ = pushCmd.RegisterFlagCompletionFunc(compressionLevel, completion.AutocompleteNone)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
	}
}

func push(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(manifestPushOpts.Authfile); err != nil {
			return err
		}
	}
	listImageSpec := args[0]
	destSpec := args[len(args)-1]
	if listImageSpec == "" {
		return fmt.Errorf(`invalid image name "%s"`, listImageSpec)
	}
	if destSpec == "" {
		return fmt.Errorf(`invalid destination "%s"`, destSpec)
	}

	if manifestPushOpts.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(manifestPushOpts.CredentialsCLI)
		if err != nil {
			return err
		}
		manifestPushOpts.Username = creds.Username
		manifestPushOpts.Password = creds.Password
	}

	if !manifestPushOpts.Quiet {
		manifestPushOpts.Writer = os.Stderr
	}

	signingCleanup, err := common.PrepareSigning(&manifestPushOpts.ImagePushOptions, &manifestPushOpts.signing)
	if err != nil {
		return err
	}
	defer signingCleanup()

	if cmd.Flags().Changed("retry") {
		retry, err := cmd.Flags().GetUint("retry")
		if err != nil {
			return err
		}

		manifestPushOpts.Retry = &retry
	}

	if cmd.Flags().Changed("retry-delay") {
		val, err := cmd.Flags().GetString("retry-delay")
		if err != nil {
			return err
		}

		manifestPushOpts.RetryDelay = val
	}

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		manifestPushOpts.SkipTLSVerify = types.NewOptionalBool(!manifestPushOpts.TLSVerifyCLI)
	}
	if cmd.Flags().Changed("insecure") {
		if manifestPushOpts.SkipTLSVerify != types.OptionalBoolUndefined {
			return errors.New("--insecure may not be used with --tls-verify")
		}
		manifestPushOpts.SkipTLSVerify = types.NewOptionalBool(manifestPushOpts.Insecure)
	}

	if cmd.Flags().Changed("compression-level") {
		val, err := cmd.Flags().GetInt("compression-level")
		if err != nil {
			return err
		}
		manifestPushOpts.CompressionLevel = &val
	}

	if cmd.Flags().Changed("compression-format") {
		if !cmd.Flags().Changed("force-compression") {
			// If `compression-format` is set and no value for `--force-compression`
			// is selected then defaults to `true`.
			manifestPushOpts.ForceCompressionFormat = true
		}
	}

	digest, err := registry.ImageEngine().ManifestPush(registry.Context(), listImageSpec, destSpec, manifestPushOpts.ImagePushOptions)
	if err != nil {
		return err
	}
	if manifestPushOpts.DigestFile != "" {
		if err := os.WriteFile(manifestPushOpts.DigestFile, []byte(digest), 0o644); err != nil {
			return err
		}
	}

	return nil
}
