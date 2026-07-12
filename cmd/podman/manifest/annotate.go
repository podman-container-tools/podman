package manifest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// manifestAnnotateOptsWrapper wraps entities.ManifestAnnotateOptions and
// prevents us from having to add CLI-only fields to the API types.
type manifestAnnotateOptsWrapper struct {
	entities.ManifestAnnotateOptions
	annotations []string
	index       bool
}

var (
	manifestAnnotateOpts = manifestAnnotateOptsWrapper{}
	annotateCmd          = &cobra.Command{
		Use:               "annotate [options] LIST IMAGEORARTIFACT",
		Short:             "Add or update information about an entry in a manifest list or image index",
		Long:              "Adds or updates information about an entry in a manifest list or image index.",
		RunE:              annotate,
		Args:              cobra.RangeArgs(1, 2),
		Example:           `podman manifest annotate --annotation left=right mylist:v1.11 sha256:15352d97781ffdf357bf3459c037be3efac4133dc9070c2dce7eca7c05c3e736`,
		ValidArgsFunction: common.AutocompleteImages,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: annotateCmd,
		Parent:  manifestCmd,
	})
	flags := annotateCmd.Flags()

	annotationFlagName := "annotation"
	flags.StringArrayVar(&manifestAnnotateOpts.annotations, annotationFlagName, nil, "set an `annotation` for the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

	archFlagName := "arch"
	flags.StringVar(&manifestAnnotateOpts.Arch, archFlagName, "", "override the `architecture` of the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

	featuresFlagName := "features"
	flags.StringSliceVar(&manifestAnnotateOpts.Features, featuresFlagName, nil, "override the `features` of the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(featuresFlagName, completion.AutocompleteNone)

	indexFlagName := "index"
	flags.BoolVar(&manifestAnnotateOpts.index, indexFlagName, false, "apply --"+annotationFlagName+" values to the image index itself")

	osFlagName := "os"
	flags.StringVar(&manifestAnnotateOpts.OS, osFlagName, "", "override the `OS` of the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

	osFeaturesFlagName := "os-features"
	flags.StringSliceVar(&manifestAnnotateOpts.OSFeatures, osFeaturesFlagName, nil, "override the OS `features` of the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(osFeaturesFlagName, completion.AutocompleteNone)

	osVersionFlagName := "os-version"
	flags.StringVar(&manifestAnnotateOpts.OSVersion, osVersionFlagName, "", "override the OS `version` of the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(osVersionFlagName, completion.AutocompleteNone)

	variantFlagName := "variant"
	flags.StringVar(&manifestAnnotateOpts.Variant, variantFlagName, "", "override the `Variant` of the specified image or artifact")
	_ = annotateCmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

	subjectFlagName := "subject"
	flags.StringVar(&manifestAnnotateOpts.IndexSubject, subjectFlagName, "", "set the `subject` to which the image index refers")
	_ = annotateCmd.RegisterFlagCompletionFunc(subjectFlagName, completion.AutocompleteNone)
}

func annotate(_ *cobra.Command, args []string) error {
	var listImageSpec, instanceSpec string
	switch len(args) {
	case 1:
		listImageSpec = args[0]
		if listImageSpec == "" {
			return fmt.Errorf(`invalid image name "%s"`, args[0])
		}
		if !manifestAnnotateOpts.index {
			return errors.New(`expected an instance digest, image name, or artifact name`)
		}
	case 2:
		listImageSpec = args[0]
		if listImageSpec == "" {
			return fmt.Errorf(`invalid image name "%s"`, args[0])
		}
		if manifestAnnotateOpts.index {
			return fmt.Errorf(`did not expect image or artifact name "%s" when modifying the entire index`, args[1])
		}
		instanceSpec = args[1]
		if instanceSpec == "" {
			return fmt.Errorf(`invalid instance digest, image name, or artifact name "%s"`, instanceSpec)
		}
	default:
		return errors.New("expected either a list name and --index or a list name and an image digest or image name or artifact name")
	}
	opts := manifestAnnotateOpts.ManifestAnnotateOptions
	var annotations map[string]string
	for _, annotation := range manifestAnnotateOpts.annotations {
		k, v, parsed := strings.Cut(annotation, "=")
		if !parsed {
			return fmt.Errorf("expected --annotation %q to be in key=value format", annotation)
		}
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[k] = v
	}
	if manifestAnnotateOpts.index {
		opts.IndexAnnotations = annotations
	} else {
		opts.Annotations = annotations
	}
	id, err := registry.ImageEngine().ManifestAnnotate(registry.Context(), listImageSpec, instanceSpec, opts)
	if err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}
