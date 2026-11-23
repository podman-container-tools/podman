//go:build unix

package dump

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
	"go.podman.io/storage/pkg/chunked/internal/minimal"
	storagePath "go.podman.io/storage/pkg/chunked/internal/path"
)

// LayerFlatFileNames maps the name of a tar entry, cleaned with
// filepath.Clean, to the name its content is stored under when the layer is
// applied in the flat layout (graphdriver.DifferOutputFormatFlat): under a
// name derived from its digest rather than under its own name.  That is the
// layout a layer pulled partially for composefs uses.
//
// bigData reads the layer big data item stored under key, failing with an
// error matching os.ErrNotExist if the layer has none.  The mapping comes
// from the layer's table of contents, which is stored there, so it is nil
// for a layer that has none.  Having one does not mean the layer was stored
// flat: a layer pulled partially without composefs keeps its files under
// their own names, and only the graph driver knows which layout it used.
//
// This is an internal implementation detail of c/storage and must not be called from outside of c/storage.
// It may be changed or removed at any time.
func LayerFlatFileNames(bigData func(key string) (io.ReadCloser, error)) (map[string]string, error) {
	r, err := bigData(minimal.ManifestBigDataKey)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer r.Close()

	// The table of contents was validated when the layer was pulled.
	var toc minimal.TOC
	if err := json.NewDecoder(r).Decode(&toc); err != nil {
		return nil, fmt.Errorf("parsing the table of contents: %w", err)
	}

	names := make(map[string]string, len(toc.Entries))
	for _, entry := range toc.Entries {
		if entry.Type != minimal.TypeReg || entry.Digest == "" {
			continue
		}
		d, err := digest.Parse(entry.Digest)
		if err != nil {
			return nil, err
		}
		p, err := storagePath.RegularFilePathForValidatedDigest(d)
		if err != nil {
			return nil, err
		}
		names[filepath.Clean(entry.Name)] = p
	}
	return names, nil
}
