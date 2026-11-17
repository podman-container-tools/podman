package imagebuildah

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah/define"
	"go.podman.io/buildah/internal/tmpdir"
	"go.podman.io/buildah/pkg/overlay"
	"go.podman.io/storage"
	"golang.org/x/sys/unix"
)

// platformSetupContextDirectoryOverlay() sets up an overlay _over_ the build
// context directory, and sorts out labeling.  Returns the location which
// should be used as the default build context; the process label and mount
// label for the build, if any; a boolean value that indicates whether we did,
// in fact, mount an overlay; and a cleanup function which should be called
// when the location is no longer needed (on success). Returned errors should
// be treated as fatal.
func platformSetupContextDirectoryOverlay(store storage.Store, options *define.BuildOptions) (string, string, string, bool, func(), error) {
	var succeeded bool
	var tmpDir, contentDir string
	cleanup := func() {
		if contentDir != "" {
			if err := overlay.CleanupContent(tmpDir); err != nil {
				logrus.Debugf("cleaning up overlay scaffolding for build context directory: %v", err)
			}
		}
		if tmpDir != "" {
			if err := os.Remove(tmpDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
				logrus.Debugf("removing should-be-empty temporary directory %q: %v", tmpDir, err)
			}
		}
	}
	defer func() {
		if !succeeded {
			cleanup()
		}
	}()
	// double-check that the context directory location is an absolute path
	contextDirectoryAbsolute, err := filepath.Abs(options.ContextDirectory)
	if err != nil {
		return "", "", "", false, nil, fmt.Errorf("determining absolute path of %q: %w", options.ContextDirectory, err)
	}
	var st unix.Stat_t
	if err := unix.Stat(contextDirectoryAbsolute, &st); err != nil {
		return "", "", "", false, nil, fmt.Errorf("checking ownership of %q: %w", options.ContextDirectory, err)
	}
	// figure out the labeling situation
	processLabel, mountLabel, err := label.InitLabels(options.CommonBuildOpts.LabelOpts)
	if err != nil {
		return "", "", "", false, nil, err
	}
	// create a temporary directory
	tmpDir, err = os.MkdirTemp(tmpdir.GetTempDir(), "buildah-context-")
	if err != nil {
		return "", "", "", false, nil, fmt.Errorf("creating temporary directory: %w", err)
	}
	// create the scaffolding for an overlay mount under it
	contentDir, err = overlay.TempDir(tmpDir, 0, 0)
	if err != nil {
		return "", "", "", false, nil, fmt.Errorf("creating overlay scaffolding for build context directory: %w", err)
	}
	// mount an overlay that uses it as a lower
	overlayOptions := overlay.Options{
		GraphOpts:  slices.Clone(store.GraphOptions()),
		ForceMount: true,
		MountLabel: mountLabel,
	}
	targetDir := filepath.Join(contentDir, "target")
	contextDirMountSpec, err := overlay.MountWithOptions(contentDir, contextDirectoryAbsolute, targetDir, &overlayOptions)
	if err != nil {
		return "", "", "", false, nil, fmt.Errorf("creating overlay scaffolding for build context directory: %w", err)
	}
	// going forward, pretend that the merged directory is the actual context directory
	logrus.Debugf("mounted an overlay at %q over %q", contextDirMountSpec.Source, contextDirectoryAbsolute)
	succeeded = true
	return contextDirMountSpec.Source, processLabel, mountLabel, true, cleanup, nil
}
