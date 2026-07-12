package images

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/buildah/pkg/cli"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// pullOptionsWrapper wraps entities.ImagePullOptions and prevents leaking
// CLI-only fields into the API types.
type pullOptionsWrapper struct {
	entities.ImagePullOptions
	TLSVerifyCLI   bool // CLI only
	CredentialsCLI string
	DecryptionKeys []string
	PolicyCLI      string
}

var (
	pullOptions     = pullOptionsWrapper{}
	pullDescription = `Pulls an image from a registry and stores it locally.

  An image can be pulled by tag or digest. If a tag is not specified, the image with the 'latest' tag is pulled.`

	// Command: podman pull
	pullCmd = &cobra.Command{
		Use:               "pull [options] IMAGE [IMAGE...]",
		Args:              cobra.MinimumNArgs(1),
		Short:             "Pull an image from a registry",
		Long:              pullDescription,
		RunE:              imagePull,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman pull imageName
podman pull fedora:latest`,
	}

	// Command: podman image pull
	// It's basically a clone of `pullCmd` with the exception of being a
	// child of the images command.
	imagesPullCmd = &cobra.Command{
		Use:               pullCmd.Use,
		Args:              pullCmd.Args,
		Short:             pullCmd.Short,
		Long:              pullCmd.Long,
		RunE:              pullCmd.RunE,
		ValidArgsFunction: pullCmd.ValidArgsFunction,
		Example: `podman image pull imageName
podman image pull fedora:latest`,
	}
)

func init() {
	// pull
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pullCmd,
	})
	pullFlags(pullCmd)

	// images pull
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imagesPullCmd,
		Parent:  imageCmd,
	})
	pullFlags(imagesPullCmd)
}

// pullFlags set the flags for the pull command.
func pullFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&pullOptions.AllTags, "all-tags", "a", false, "All tagged images in the repository will be pulled")

	credsFlagName := "creds"
	flags.StringVar(&pullOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	archFlagName := "arch"
	flags.StringVar(&pullOptions.Arch, archFlagName, "", "Use `ARCH` instead of the architecture of the machine for choosing images")
	_ = cmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

	osFlagName := "os"
	flags.StringVar(&pullOptions.OS, osFlagName, "", "Use `OS` instead of the running OS for choosing images")
	_ = cmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

	variantFlagName := "variant"
	flags.StringVar(&pullOptions.Variant, variantFlagName, "", "Use VARIANT instead of the running architecture variant for choosing images")
	_ = cmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

	platformFlagName := "platform"
	flags.String(platformFlagName, "", "Specify the platform for selecting the image.  (Conflicts with arch and os)")
	_ = cmd.RegisterFlagCompletionFunc(platformFlagName, completion.AutocompleteNone)

	policyFlagName := "policy"
	// Explicitly set the default to "always" to avoid the default being "missing"
	flags.StringVar(&pullOptions.PolicyCLI, policyFlagName, "always", `Pull image policy ("always"|"missing"|"never"|"newer")`)
	_ = cmd.RegisterFlagCompletionFunc(policyFlagName, common.AutocompletePullOption)

	flags.Bool("disable-content-trust", false, "This is a Docker specific option and is a NOOP")
	flags.BoolVarP(&pullOptions.Quiet, "quiet", "q", false, "Suppress output information when pulling images")
	flags.BoolVar(&pullOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")

	authfileFlagName := "authfile"
	flags.StringVar(&pullOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
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
		flags.StringVar(&pullOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
		_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

		signaturePolicyFlagName := "signature-policy"
		flags.StringVar(&pullOptions.SignaturePolicy, signaturePolicyFlagName, "", "`Pathname` of signature policy file (not usually used)")
		_ = flags.MarkHidden(signaturePolicyFlagName)
	}
}

// imagePull is implement the command for pulling images.
func imagePull(cmd *cobra.Command, args []string) error {
	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		pullOptions.SkipTLSVerify = types.NewOptionalBool(!pullOptions.TLSVerifyCLI)
	}

	pullPolicy, err := config.ParsePullPolicy(pullOptions.PolicyCLI)
	if err != nil {
		return err
	}
	pullOptions.PullPolicy = pullPolicy

	if cmd.Flags().Changed("retry") {
		retry, err := cmd.Flags().GetUint("retry")
		if err != nil {
			return err
		}

		pullOptions.Retry = &retry
	}

	if cmd.Flags().Changed("retry-delay") {
		val, err := cmd.Flags().GetString("retry-delay")
		if err != nil {
			return err
		}

		pullOptions.RetryDelay = val
	}

	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(pullOptions.Authfile); err != nil {
			return err
		}
	}
	platform, err := cmd.Flags().GetString("platform")
	if err != nil {
		return err
	}
	if platform != "" {
		if pullOptions.Arch != "" || pullOptions.OS != "" {
			return errors.New("--platform option can not be specified with --arch or --os")
		}

		specs := strings.Split(platform, "/")
		pullOptions.OS = specs[0] // may be empty
		if len(specs) > 1 {
			pullOptions.Arch = specs[1]
			if len(specs) > 2 {
				pullOptions.Variant = specs[2]
			}
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
		pullOptions.Writer = os.Stderr
	}

	// Let's do all the remaining Yoga in the API to prevent us from
	// scattering logic across (too) many parts of the code.
	var errs utils.OutputErrors
	for _, arg := range args {
		pullReport, err := registry.ImageEngine().Pull(registry.Context(), arg, pullOptions.ImagePullOptions)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, img := range pullReport.Images {
			fmt.Println(img)
		}
	}
	return errs.PrintErrors()
}
