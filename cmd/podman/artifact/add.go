package artifact

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/utils"
)

var addCmd = &cobra.Command{
	Use:               "add [options] ARTIFACT PATH [...PATH]",
	Short:             "Add an OCI artifact to the local store",
	Long:              "Add an OCI artifact to the local store from the local filesystem",
	RunE:              add,
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: common.AutocompleteArtifactAdd,
	Example: `podman artifact add quay.io/myimage/myartifact:latest /tmp/foobar.txt
podman artifact add --file-type text/yaml quay.io/myimage/myartifact:latest /tmp/foobar.yaml
podman artifact add --append quay.io/myimage/myartifact:latest /tmp/foobar.tar.gz`,
}

// AddOptionsWrapper wraps entities.ArtifactsAddOptions and prevents leaking
// CLI-only fields into the API types.
type AddOptionsWrapper struct {
	entities.ArtifactAddOptions
	AnnotationsCLI []string // CLI only
}

var addOpts AddOptionsWrapper

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: addCmd,
		Parent:  artifactCmd,
	})
	flags := addCmd.Flags()

	annotationFlagName := "annotation"
	flags.StringArrayVar(&addOpts.AnnotationsCLI, annotationFlagName, nil, "set an `annotation` for the specified files of artifact")
	_ = addCmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

	addMIMETypeFlagName := "type"
	flags.StringVar(&addOpts.ArtifactMIMEType, addMIMETypeFlagName, "", "Use type to describe an artifact")
	_ = addCmd.RegisterFlagCompletionFunc(addMIMETypeFlagName, completion.AutocompleteNone)

	appendFlagName := "append"
	flags.BoolVarP(&addOpts.Append, appendFlagName, "a", false, "Append files to an existing artifact")

	replaceFlagName := "replace"
	flags.BoolVar(&addOpts.Replace, replaceFlagName, false, "Replace an existing artifact")

	fileMIMETypeFlagName := "file-type"
	flags.StringVarP(&addOpts.FileMIMEType, fileMIMETypeFlagName, "", "", "Set file type to use for the artifact (layer)")
	_ = addCmd.RegisterFlagCompletionFunc(fileMIMETypeFlagName, completion.AutocompleteNone)
}

func add(_ *cobra.Command, args []string) error {
	artifactName := args[0]
	blobs := args[1:]

	if addOpts.Append && addOpts.Replace {
		return fmt.Errorf("--append and --replace options cannot be used together")
	}

	annots, err := utils.ParseAnnotations(addOpts.AnnotationsCLI)
	if err != nil {
		return err
	}

	opts := entities.ArtifactAddOptions{
		Annotations:      annots,
		ArtifactMIMEType: addOpts.ArtifactMIMEType,
		Append:           addOpts.Append,
		FileMIMEType:     addOpts.FileMIMEType,
		Replace:          addOpts.Replace,
	}

	artifactBlobs := make([]entities.ArtifactBlob, 0, len(blobs))

	for _, blobPath := range blobs {
		artifactBlob := entities.ArtifactBlob{
			BlobFilePath: blobPath,
			FileName:     filepath.Base(blobPath),
		}

		artifactBlobs = append(artifactBlobs, artifactBlob)
	}

	report, err := registry.ImageEngine().ArtifactAdd(registry.Context(), artifactName, artifactBlobs, opts)
	if err != nil {
		return err
	}
	fmt.Println(report.ArtifactDigest.Encoded())
	return nil
}
