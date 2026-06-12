package manifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// manifestAddOptsWrapper wraps entities.ManifestAddOptions and prevents
// leaking CLI-only fields into the API types.
type manifestAddOptsWrapper struct {
	entities.ManifestAddOptions
	artifactOptions entities.ManifestAddArtifactOptions

	tlsVerifyCLI        bool     // CLI only
	insecure            bool     // CLI only
	credentialsCLI      string   // CLI only
	artifact            bool     // CLI only
	artifactConfigFile  string   // CLI only
	artifactType        string   // CLI only
	artifactAnnotations []string // CLI only
}

var (
	manifestAddOpts = manifestAddOptsWrapper{}
	addCmd          = &cobra.Command{
		Use:               "add [options] LIST IMAGEORARTIFACT [IMAGEORARTIFACT...]",
		Short:             "Add images or artifacts to a manifest list or image index",
		Long:              "Adds an image or artifact to a manifest list or image index.",
		RunE:              add,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman manifest add mylist:v1.11 image:v1.11-amd64
podman manifest add mylist:v1.11 transport:imageName`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: addCmd,
		Parent:  manifestCmd,
	})
	flags := addCmd.Flags()
	flags.BoolVar(&manifestAddOpts.All, "all", false, "add all of the list's images if the image is a list")

	annotationFlagName := "annotation"
	flags.StringArrayVar(&manifestAddOpts.Annotation, annotationFlagName, nil, "set an `annotation` for the specified image or artifact")
	_ = addCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

	archFlagName := "arch"
	flags.StringVar(&manifestAddOpts.Arch, archFlagName, "", "override the `architecture` of the specified image or artifact")
	_ = addCmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

	artifactFlagName := "artifact"
	flags.BoolVar(&manifestAddOpts.artifact, artifactFlagName, false, "add all arguments as artifact files rather than as images")

	artifactAnnotationFlagName := "artifact-annotation"
	flags.StringArrayVar(&manifestAddOpts.artifactAnnotations, artifactAnnotationFlagName, nil, "set an `annotation` in the artifact")
	_ = addCmd.RegisterFlagCompletionFunc(artifactAnnotationFlagName, completion.AutocompleteNone)

	artifactExcludeTitlesFlagName := "artifact-exclude-titles"
	flags.BoolVar(&manifestAddOpts.artifactOptions.ExcludeTitles, artifactExcludeTitlesFlagName, false, fmt.Sprintf(`refrain from setting %q annotations on "layers"`, imgspecv1.AnnotationTitle))

	artifactTypeFlagName := "artifact-type"
	flags.StringVar(&manifestAddOpts.artifactType, artifactTypeFlagName, "", "override the artifactType value")
	_ = addCmd.RegisterFlagCompletionFunc(artifactTypeFlagName, completion.AutocompleteNone)

	artifactConfigFlagName := "artifact-config"
	flags.StringVar(&manifestAddOpts.artifactConfigFile, artifactConfigFlagName, "", "artifact configuration file")
	_ = addCmd.RegisterFlagCompletionFunc(artifactConfigFlagName, completion.AutocompleteNone)

	artifactConfigTypeFlagName := "artifact-config-type"
	flags.StringVar(&manifestAddOpts.artifactOptions.ConfigType, artifactConfigTypeFlagName, "", "artifact configuration media type")
	_ = addCmd.RegisterFlagCompletionFunc(artifactConfigTypeFlagName, completion.AutocompleteNone)

	artifactLayerTypeFlagName := "artifact-layer-type"
	flags.StringVar(&manifestAddOpts.artifactOptions.LayerType, artifactLayerTypeFlagName, "", "artifact layer media type")
	_ = addCmd.RegisterFlagCompletionFunc(artifactLayerTypeFlagName, completion.AutocompleteNone)

	artifactSubjectFlagName := "artifact-subject"
	flags.StringVar(&manifestAddOpts.IndexSubject, artifactSubjectFlagName, "", "artifact subject reference")
	_ = addCmd.RegisterFlagCompletionFunc(artifactSubjectFlagName, completion.AutocompleteNone)

	authfileFlagName := "authfile"
	flags.StringVar(&manifestAddOpts.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = addCmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	certDirFlagName := "cert-dir"
	flags.StringVar(&manifestAddOpts.CertDir, certDirFlagName, "", "use certificates at the specified path to access the registry")
	_ = addCmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)

	credsFlagName := "creds"
	flags.StringVar(&manifestAddOpts.credentialsCLI, credsFlagName, "", "use `[username[:password]]` for accessing the registry")
	_ = addCmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	featuresFlagName := "features"
	flags.StringSliceVar(&manifestAddOpts.Features, featuresFlagName, nil, "override the `features` of the specified image or artifact")
	_ = addCmd.RegisterFlagCompletionFunc(featuresFlagName, completion.AutocompleteNone)

	osFlagName := "os"
	flags.StringVar(&manifestAddOpts.OS, osFlagName, "", "override the `OS` of the specified image or artifact")
	_ = addCmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

	osVersionFlagName := "os-version"
	flags.StringVar(&manifestAddOpts.OSVersion, osVersionFlagName, "", "override the OS `version` of the specified image or artifact")
	_ = addCmd.RegisterFlagCompletionFunc(osVersionFlagName, completion.AutocompleteNone)

	flags.BoolVar(&manifestAddOpts.insecure, "insecure", false, "neither require HTTPS nor verify certificates when accessing the registry")
	_ = flags.MarkHidden("insecure")
	flags.BoolVar(&manifestAddOpts.tlsVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")

	variantFlagName := "variant"
	flags.StringVar(&manifestAddOpts.Variant, variantFlagName, "", "override the `variant` of the specified image or artifact")
	_ = addCmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cert-dir")
	}
}

func add(cmd *cobra.Command, args []string) error {
	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(manifestAddOpts.Authfile); err != nil {
			return err
		}
	}

	if manifestAddOpts.credentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(manifestAddOpts.credentialsCLI)
		if err != nil {
			return err
		}
		manifestAddOpts.Username = creds.Username
		manifestAddOpts.Password = creds.Password
	}

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		manifestAddOpts.SkipTLSVerify = types.NewOptionalBool(!manifestAddOpts.tlsVerifyCLI)
	}
	if cmd.Flags().Changed("insecure") {
		if manifestAddOpts.SkipTLSVerify != types.OptionalBoolUndefined {
			return errors.New("--insecure may not be used with --tls-verify")
		}
		manifestAddOpts.SkipTLSVerify = types.NewOptionalBool(manifestAddOpts.insecure)
	}

	if !manifestAddOpts.artifact {
		var changedArtifactFlags []string
		for _, artifactOption := range []string{"artifact-type", "artifact-config", "artifact-config-type", "artifact-layer-type", "artifact-subject", "artifact-exclude-titles", "artifact-annotation"} {
			if cmd.Flags().Changed(artifactOption) {
				changedArtifactFlags = append(changedArtifactFlags, "--"+artifactOption)
			}
		}
		switch {
		case len(changedArtifactFlags) == 1:
			return fmt.Errorf("%s requires --artifact", changedArtifactFlags[0])
		case len(changedArtifactFlags) > 1:
			return fmt.Errorf("%s require --artifact", strings.Join(changedArtifactFlags, "/"))
		}
	}

	var listID string
	var err error
	if manifestAddOpts.artifact {
		if cmd.Flags().Changed("artifact-type") {
			manifestAddOpts.artifactOptions.Type = &manifestAddOpts.artifactType
		}
		if manifestAddOpts.artifactConfigFile != "" {
			configBytes, err := os.ReadFile(manifestAddOpts.artifactConfigFile)
			if err != nil {
				return fmt.Errorf("%v", err)
			}
			manifestAddOpts.artifactOptions.Config = string(configBytes)
		}
		if len(manifestAddOpts.artifactAnnotations) > 0 {
			manifestAddOpts.artifactOptions.Annotations = make(map[string]string, len(manifestAddOpts.artifactAnnotations))
			for _, annotation := range manifestAddOpts.artifactAnnotations {
				key, val, hasVal := strings.Cut(annotation, "=")
				if !hasVal {
					return fmt.Errorf("no value given for annotation %q", key)
				}
				manifestAddOpts.artifactOptions.Annotations[key] = val
			}
		}
		manifestAddOpts.artifactOptions.ManifestAnnotateOptions = manifestAddOpts.ManifestAnnotateOptions
		listID, err = registry.ImageEngine().ManifestAddArtifact(context.Background(), args[0], args[1:], manifestAddOpts.artifactOptions)
		if err != nil {
			return err
		}
	} else {
		listID, err = registry.ImageEngine().ManifestAdd(context.Background(), args[0], args[1:], manifestAddOpts.ManifestAddOptions)
		if err != nil {
			return err
		}
	}
	fmt.Println(listID)
	return nil
}
