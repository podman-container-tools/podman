//go:build !linux

package imagebuildah

import (
	"go.podman.io/buildah/define"
	"go.podman.io/storage"
)

// platformSetupContextDirectoryOverlay() should set up an overlay _over_ the
// build context directory, and sort out labeling.  Should return the location
// which should be used as the default build context; the process label and
// mount label for the build, if any; a boolean value that indicates whether we
// did, in fact, mount an overlay; and a cleanup function which should be
// called when the location is no longer needed (on success).  Returned errors
// should be treated as fatal.
// TODO: currenty a no-op on this platform.
func platformSetupContextDirectoryOverlay(store storage.Store, options *define.BuildOptions) (string, string, string, bool, func(), error) {
	return options.ContextDirectory, "", "", false, func() {}, nil
}
