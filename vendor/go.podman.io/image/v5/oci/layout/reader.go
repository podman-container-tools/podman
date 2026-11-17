package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/image/v5/types"
)

// Reader manages an OCI layout.
//
// Many users don’t need this and can use NewReference… to the directory directly.
type Reader struct {
	root *os.Root
}

// NewReaderWithRoot creates a Reader where all ImageSource operations are restricted to the given root.
//
// The root must not be closed as long as references created by this Reader exist.
func NewReaderWithRoot(root *os.Root) *Reader {
	return &Reader{root: root}
}

// NewReference returns an OCI reference for a directory and an optional image name annotation (if not "").
//
// dir must match Reader’s root (as determined by root.Name()). This may be relaxed in the future.
func (r *Reader) NewReference(dir, image string) (types.ImageReference, error) {
	return newReference(dir, image, -1, r)
}

// NewIndexReference returns an OCI reference for a directory and a zero-based source manifest index.
//
// dir must match Reader’s root (as determined by root.Name()). This may be relaxed in the future.
func (r *Reader) NewIndexReference(dir string, sourceIndex int) (types.ImageReference, error) {
	if sourceIndex < 0 {
		return nil, fmt.Errorf("invalid call to NewIndexReference with negative index %d", sourceIndex)
	}
	return newReference(dir, "", sourceIndex, r)
}

// ListResult wraps the image reference and the manifest for loading
type ListResult struct {
	Reference          types.ImageReference
	ManifestDescriptor imgspecv1.Descriptor
}

// List returns a slice of manifests included in the archive
func List(dir string) ([]ListResult, error) {
	var res []ListResult

	indexJSON, err := os.ReadFile(filepath.Join(dir, imgspecv1.ImageIndexFile))
	if err != nil {
		return nil, err
	}
	var index imgspecv1.Index
	if err := json.Unmarshal(indexJSON, &index); err != nil {
		return nil, err
	}

	for manifestIndex, md := range index.Manifests {
		refName := md.Annotations[imgspecv1.AnnotationRefName]
		index := -1
		if refName == "" {
			index = manifestIndex
		}
		ref, err := newReference(dir, refName, index, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating image reference: %w", err)
		}
		reference := ListResult{
			Reference:          ref,
			ManifestDescriptor: md,
		}
		res = append(res, reference)
	}
	return res, nil
}
