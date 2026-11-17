package volumes

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah/copier"
	"go.podman.io/buildah/define"
	"go.podman.io/buildah/internal"
	internalParse "go.podman.io/buildah/internal/parsevolume"
	"go.podman.io/buildah/internal/tmpdir"
	internalUtil "go.podman.io/buildah/internal/util"
	"go.podman.io/buildah/pkg/overlay"
	"go.podman.io/buildah/util"
	"go.podman.io/common/pkg/parse"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage"
	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/lockfile"
	"go.podman.io/storage/pkg/mount"
	"go.podman.io/storage/pkg/unshare"
)

const (
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
	// TypeCache is the type for mounting a common persistent cache from host
	TypeCache = "cache"
	// mount=type=cache must create a persistent directory on host so its available for all consecutive builds.
	// Lifecycle of following directory will be inherited from how host machine treats temporary directory
	buildahCacheDir = "buildah-cache"
	// mount=type=cache allows users to lock a cache store while its being used by another build
	BuildahCacheLockfile = "buildah-cache-lockfile"
	// All the lockfiles are stored in a separate directory inside `BuildahCacheDir`
	// Example `/var/tmp/buildah-cache/<target>/buildah-cache-lockfile`
	BuildahCacheLockfileDir = "buildah-cache-lockfiles"
	// sharing=private holds this lock while it picks a directory for the cache
	buildahCachePoolLockfile = "buildah-cache-pool-lockfile"
)

var (
	errBadMntOption   = errors.New("invalid mount option")
	errBadOptionArg   = errors.New("must provide an argument for option")
	errBadOptionNoArg = errors.New("must not provide an argument for option")
	errBadVolDest     = errors.New("must set volume destination")
	errBadVolSrc      = errors.New("must set volume source")
	errDuplicateDest  = errors.New("duplicate mount destination")
)

// CacheParent returns a cache parent for --mount=type=cache
func CacheParent() string {
	return filepath.Join(tmpdir.GetTempDir(), buildahCacheDir+"-"+strconv.Itoa(unshare.GetRootlessUID()))
}

func mountIsReadWrite(m specs.Mount) bool {
	// in case of conflicts, the last one wins, so it's not enough
	// to check for the presence of either "rw" or "ro" anywhere
	// with e.g. slices.Contains()
	rw := true
	for _, option := range m.Options {
		switch option {
		case "rw":
			rw = true
		case "ro":
			rw = false
		}
	}
	return rw
}

func convertToOverlay(m specs.Mount, store storage.Store, mountLabel, tmpDir string, uid, gid int) (specs.Mount, string, error) {
	overlayDir, err := overlay.TempDir(tmpDir, uid, gid)
	if err != nil {
		return specs.Mount{}, "", fmt.Errorf("setting up overlay for %q: %w", m.Destination, err)
	}
	options := overlay.Options{GraphOpts: slices.Clone(store.GraphOptions()), ForceMount: true, MountLabel: mountLabel}
	fileInfo, err := os.Stat(m.Source)
	if err != nil {
		return specs.Mount{}, "", fmt.Errorf("setting up overlay of %q: %w", m.Source, err)
	}
	// we might be trying to "overlay" for a non-directory, and the kernel doesn't like that very much
	var mountThisInstead specs.Mount
	if fileInfo.IsDir() {
		// do the normal thing of mounting this directory as a lower with a temporary upper
		mountThisInstead, err = overlay.MountWithOptions(overlayDir, m.Source, m.Destination, &options)
		if err != nil {
			return specs.Mount{}, "", fmt.Errorf("setting up overlay of %q: %w", m.Source, err)
		}
	} else {
		// mount the parent directory as the lower with a temporary upper, and return a
		// bind mount from the non-directory in the merged directory to the destination
		sourceDir := filepath.Dir(m.Source)
		sourceBase := filepath.Base(m.Source)
		destination := m.Destination
		mountedOverlay, err := overlay.MountWithOptions(overlayDir, sourceDir, destination, &options)
		if err != nil {
			return specs.Mount{}, "", fmt.Errorf("setting up overlay of %q: %w", sourceDir, err)
		}
		if mountedOverlay.Type != define.TypeBind {
			if err2 := overlay.RemoveTemp(overlayDir); err2 != nil {
				return specs.Mount{}, "", fmt.Errorf("cleaning up after failing to set up overlay for %q: %w", destination, err2)
			}
			return specs.Mount{}, "", fmt.Errorf("setting up overlay for %q at %q, incorrect mount type: %q (expected \"bind\")", mountedOverlay.Source, destination, mountedOverlay.Type)
		}
		mountThisInstead = mountedOverlay
		mountThisInstead.Source = filepath.Join(mountedOverlay.Source, sourceBase)
		mountThisInstead.Destination = destination
	}
	return mountThisInstead, overlayDir, nil
}

// FIXME: this code needs to be merged with pkg/parse/parse.go ValidateVolumeOpts
//
// GetBindMount parses a single bind mount entry from the --mount flag.
//
// Returns a Mount to add to the runtime spec's list of mounts, the ID of the
// image we mounted if we mounted one, the path of a mounted location if one
// needs to be unmounted and removed, and the path of an overlay mount if one
// needs to be cleaned up, or an error.
//
// The caller is expected to, after the command which uses the mount exits,
// clean up the overlay filesystem (if we provided a path to it), unmount and
// remove the mountpoint for the mounted filesystem (if we provided the path to
// its mountpoint), and then unmount the image (if we mounted one).
func GetBindMount(ctx context.Context, sys *types.SystemContext, args []string, contextDir string, store storage.Store, mountLabel string, additionalMountPoints map[string]internal.StageMountDetails, workDir, tmpDir string) (specs.Mount, string, string, string, error) {
	select {
	case <-ctx.Done():
		return specs.Mount{}, "", "", "", ctx.Err()
	default:
	}

	newMount := specs.Mount{
		Type: define.TypeBind,
	}

	setRelabel := ""
	mountReadability := ""
	setDest := ""
	bindNonRecursive := false
	fromWhere := ""
	skipOverlay := false

	for _, val := range args {
		argName, argValue, hasArgValue := strings.Cut(val, "=")
		switch argName {
		case "type":
			// This is already processed, and should be "bind"
			continue
		case "bind-nonrecursive":
			if hasArgValue {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, "bind")
			bindNonRecursive = true
		case "nosuid", "nodev", "noexec":
			if hasArgValue {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, argName)
		case "rw", "readwrite":
			if hasArgValue {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, "rw")
			mountReadability = "rw"
		case "ro", "readonly":
			if hasArgValue {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, "ro")
			mountReadability = "ro"
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "Z", "z", "U", "no-dereference":
			if hasArgValue {
				return newMount, "", "", "", fmt.Errorf("%v: %w", val, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, argName)
		case "from":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			fromWhere = argValue
		case "bind-propagation":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			switch argValue {
			default:
				return newMount, "", "", "", fmt.Errorf("%v: %q: %w", argName, argValue, errBadMntOption)
			case "shared", "rshared", "private", "rprivate", "slave", "rslave":
				// this should be the relevant parts of the same list of options we accepted above
			}
			newMount.Options = append(newMount.Options, argValue)
		case "src", "source":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			newMount.Source = argValue
		case "target", "dst", "destination":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			targetPath := argValue
			setDest = targetPath
			if !path.IsAbs(targetPath) {
				targetPath = filepath.Join(workDir, targetPath)
			}
			if err := parse.ValidateVolumeCtrDir(targetPath); err != nil {
				return newMount, "", "", "", err
			}
			newMount.Destination = targetPath
		case "relabel":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			if setRelabel != "" {
				return newMount, "", "", "", fmt.Errorf("cannot pass 'relabel' option more than once: %w", errBadOptionArg)
			}
			setRelabel = argValue
			switch argValue {
			case "private":
				newMount.Options = append(newMount.Options, "Z")
			case "shared":
				newMount.Options = append(newMount.Options, "z")
			default:
				return newMount, "", "", "", fmt.Errorf("%s mount option must be 'private' or 'shared': %w", argName, errBadMntOption)
			}
		case "consistency":
			// Option for OS X only, has no meaning on other platforms
			// and can thus be safely ignored.
			// See also the handling of the equivalent "delegated" and "cached" in ValidateVolumeOpts
		default:
			return newMount, "", "", "", fmt.Errorf("%v: %w", argName, errBadMntOption)
		}
	}

	// default mount readability is always readonly
	if mountReadability == "" {
		newMount.Options = append(newMount.Options, "ro")
	}

	// Following variable ensures that we return imagename only if we did additional mount
	succeeded := false
	mountedImage := ""
	if fromWhere != "" {
		mountPoint := ""
		if additionalMountPoints != nil {
			if val, ok := additionalMountPoints[fromWhere]; ok {
				mountPoint = val.MountPoint
				skipOverlay = val.IsWritesDiscardedOverlay
			}
		}
		// if mountPoint of image was not found in additionalMap
		// or additionalMap was nil, try mounting image
		if mountPoint == "" {
			image, err := internalUtil.LookupImage(sys, store, fromWhere)
			if err != nil {
				return newMount, "", "", "", err
			}

			mountPoint, err = image.Mount(context.Background(), nil, mountLabel)
			if err != nil {
				return newMount, "", "", "", err
			}
			mountedImage = image.ID()
			// unmount the image if we don't end up returning successfully
			defer func() {
				if !succeeded {
					if _, err := store.UnmountImage(mountedImage, false); err != nil {
						logrus.Debugf("unmounting bind-mounted image %q: %v", fromWhere, err)
					}
				}
			}()
		}
		contextDir = mountPoint
	} else {
		// special case an additional mount point for "" as shorthand for "preferred location of the default build context"
		if additionalMountPoints != nil {
			if val, ok := additionalMountPoints[""]; ok {
				contextDir = val.MountPoint
				skipOverlay = val.IsWritesDiscardedOverlay
			}
		}
	}

	// buildkit parity: default bind option must be `rbind`
	// unless specified
	if !bindNonRecursive {
		newMount.Options = append(newMount.Options, "rbind")
	}

	if setDest == "" {
		return newMount, "", "", "", errBadVolDest
	}

	// buildkit parity: support absolute path for sources from current build context
	if contextDir != "" {
		// path should be /contextDir/specified path
		evaluated, err := copier.EvalContext(ctx, contextDir, contextDir+string(filepath.Separator)+newMount.Source, copier.EvalOptions{})
		if err != nil {
			return newMount, "", "", "", err
		}
		newMount.Source = evaluated
	} else {
		// looks like its coming from `build run --mount=type=bind` allow using absolute path
		// error out if no source is set
		if newMount.Source == "" {
			return newMount, "", "", "", errBadVolSrc
		}
		if err := parse.ValidateVolumeHostDir(newMount.Source); err != nil {
			return newMount, "", "", "", err
		}
	}

	opts, err := parse.ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, "", "", "", err
	}
	newMount.Options = opts

	var intermediateMount string
	if contextDir != "" && newMount.Source != contextDir {
		rel, err := filepath.Rel(contextDir, newMount.Source)
		if err != nil {
			return newMount, "", "", "", fmt.Errorf("computing pathname of bind subdirectory: %w", err)
		}
		if rel != "." && rel != "/" {
			mnt, err := bindFromChroot(contextDir, rel, tmpDir)
			if err != nil {
				return newMount, "", "", "", fmt.Errorf("sanitizing bind subdirectory %q: %w", newMount.Source, err)
			}
			logrus.Debugf("bind-mounted %q under %q to %q", rel, contextDir, mnt)
			intermediateMount = mnt
			newMount.Source = intermediateMount
		}
	}

	overlayDir := ""
	if !skipOverlay && (mountedImage != "" || mountIsReadWrite(newMount)) {
		if newMount, overlayDir, err = convertToOverlay(newMount, store, mountLabel, tmpDir, 0, 0); err != nil {
			return newMount, "", "", "", err
		}
	}

	succeeded = true
	return newMount, mountedImage, intermediateMount, overlayDir, nil
}

// GetCacheMount parses a single cache mount entry from the --mount flag.
//
// Returns a Mount to add to the runtime spec's list of mounts, the ID of the
// image we mounted if we mounted one, the path of a mounted filesystem if one
// needs to be unmounted, the path of an overlay if one needs to be cleaned up,
// and an optional lock that needs to be released, or an error.
//
// The caller is expected to, after the command which uses the mount exits,
// clean up the overlay filesystem (if we provided the path of one), unmount
// and remove the mountpoint of the mounted filesystem (if we provided the path
// to its mountpoint), unmount the image (if we mounted one), and release the
// lock (if we took one).
func GetCacheMount(ctx context.Context, sys *types.SystemContext, args []string, store storage.Store, mountLabel string, additionalMountPoints map[string]internal.StageMountDetails, uidmap, gidmap []specs.LinuxIDMapping, workDir, tmpDir string) (specs.Mount, string, string, string, *lockfile.LockFile, error) {
	select {
	case <-ctx.Done():
		return specs.Mount{}, "", "", "", nil, ctx.Err()
	default:
	}

	var err error
	var mode uint64
	var setShared bool
	setDest := ""
	setRelabel := ""
	setReadOnly := ""
	fromWhere := ""
	newMount := specs.Mount{
		Type: define.TypeBind,
	}
	// if id is set a new subdirectory with `id` will be created under /host-temp/buildah-build-cache/id
	id := ""
	// buildkit parity: cache directory defaults to 0o755
	mode = 0o755
	// buildkit parity: cache directory defaults to uid 0 if not specified
	uid := uint64(0)
	// buildkit parity: cache directory defaults to gid 0 if not specified
	gid := uint64(0)
	// sharing mode
	sharing := "shared"

	for _, val := range args {
		argName, argValue, hasArgValue := strings.Cut(val, "=")
		switch argName {
		case "type":
			// This is already processed, and should be "cache"
			continue
		case "nosuid", "nodev", "noexec", "U":
			if hasArgValue {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, argName)
		case "rw", "readwrite":
			if hasArgValue {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, "rw")
			setReadOnly = "rw"
		case "readonly", "ro":
			if hasArgValue {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, "ro")
			setReadOnly = "ro"
		case "Z", "z":
			if hasArgValue {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, argName)
			setRelabel = argName
		case "shared", "rshared", "private", "rprivate", "slave", "rslave":
			if hasArgValue {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, argName)
			setShared = true
		case "sharing":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			switch argValue {
			default:
				return newMount, "", "", "", nil, fmt.Errorf("unrecognized value %q for field `sharing`: %w", argValue, errBadMntOption)
			case "shared", "private", "locked":
				// the modes we know how to handle
			}
			sharing = argValue
		case "bind-propagation":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			switch argValue {
			default:
				return newMount, "", "", "", nil, fmt.Errorf("%v: %q: %w", argName, argValue, errBadMntOption)
			case "shared", "rshared", "private", "rprivate", "slave", "rslave":
				// this should be the relevant parts of the same list of options we accepted above
			}
			newMount.Options = append(newMount.Options, argValue)
			setShared = true
		case "id":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			id = argValue
		case "from":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			fromWhere = argValue
		case "target", "dst", "destination":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			targetPath := argValue
			if !path.IsAbs(targetPath) {
				targetPath = filepath.Join(workDir, targetPath)
			}
			if err := parse.ValidateVolumeCtrDir(targetPath); err != nil {
				return newMount, "", "", "", nil, err
			}
			newMount.Destination = targetPath
			setDest = targetPath
		case "src", "source":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			newMount.Source = argValue
		case "mode":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			mode, err = strconv.ParseUint(argValue, 8, 32)
			if err != nil {
				return newMount, "", "", "", nil, fmt.Errorf("unable to parse cache mode %q: %w", argValue, err)
			}
		case "uid":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			uid, err = strconv.ParseUint(argValue, 10, 32)
			if err != nil {
				return newMount, "", "", "", nil, fmt.Errorf("unable to parse cache uid %q: %w", argValue, err)
			}
		case "gid":
			if !hasArgValue || argValue == "" {
				return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			gid, err = strconv.ParseUint(argValue, 10, 32)
			if err != nil {
				return newMount, "", "", "", nil, fmt.Errorf("unable to parse cache gid %q: %w", argValue, err)
			}
		default:
			return newMount, "", "", "", nil, fmt.Errorf("%v: %w", argName, errBadMntOption)
		}
	}

	// If selinux is enabled and no selinux option was configured
	// default to `z` i.e shared content label.
	if setRelabel == "" && (selinux.EnforceMode() != selinux.Disabled) && fromWhere == "" {
		newMount.Options = append(newMount.Options, "z")
	}

	if setDest == "" {
		return newMount, "", "", "", nil, errBadVolDest
	}

	hostUID, hostGID, err := util.GetHostIDs(uidmap, gidmap, uint32(uid), uint32(gid))
	if err != nil {
		return newMount, "", "", "", nil, err
	}

	succeeded := false
	needToOverlay := false
	mountedImage := ""
	thisCacheRoot := ""
	// where this user's cache directories and their lock files live
	cacheParent := CacheParent()
	// the ID under which this cache's lock file lives, if we need one
	lockID := ""
	var targetLock *lockfile.LockFile
	defer func() {
		if !succeeded && targetLock != nil {
			targetLock.Unlock()
		}
	}()
	if fromWhere != "" {
		// do not create and use a cache directory on the host,
		// instead use the location in the mounted stage or
		// temporary directory as the cache
		mountPoint := ""
		if additionalMountPoints != nil {
			if val, ok := additionalMountPoints[fromWhere]; ok {
				mountPoint = val.MountPoint
				needToOverlay = val.IsImage
			}
		}
		// it's not an additional build context, stage, or
		// already-mounted image, but it might still be an image
		if mountPoint == "" {
			image, err := internalUtil.LookupImage(sys, store, fromWhere)
			if err != nil {
				return newMount, "", "", "", nil, err
			}

			mountPoint, err = image.Mount(context.Background(), nil, mountLabel)
			if err != nil {
				return newMount, "", "", "", nil, err
			}
			// unmount the image if we don't end up returning successfully
			mountedImage = image.ID()
			defer func() {
				if !succeeded {
					if _, err := store.UnmountImage(mountedImage, false); err != nil {
						logrus.Debugf("unmounting image %q: %v", fromWhere, err)
					}
				}
			}()
			needToOverlay = true
		}
		thisCacheRoot = mountPoint
		lockID = digest.FromString(mountPoint).Encoded()[:16]
	} else {
		// we need to create the cache directory on the host if no stage is being used

		// since type is cache and a cache can be reused by consecutive builds
		// create a common cache directory, which persists on hosts within temp lifecycle
		// add subdirectory if specified

		// create cache on host if not present
		err = os.MkdirAll(cacheParent, os.FileMode(0o755))
		if err != nil {
			return newMount, "", "", "", nil, fmt.Errorf("unable to create build cache directory: %w", err)
		}

		// caches are told apart by their ID, or by their target if they don't
		// have one, and by their ownership
		cacheKey := fmt.Sprintf("%s:%d:%d", cmp.Or(id, newMount.Destination), uid, gid)
		// hash it, so the user can't inject path components and escape the cache
		dirID := digest.FromString(cacheKey).Encoded()[:16]

		// a private cache uses a directory which no other build is using, and
		// keeps it locked until the command we're running has finished
		if sharing == "private" {
			dirID, targetLock, err = lockPrivateCacheDir(cacheParent, dirID)
			if err != nil {
				return newMount, "", "", "", nil, err
			}
		}

		thisCacheRoot = filepath.Join(cacheParent, dirID)
		lockID = dirID

		idPair := idtools.IDPair{
			UID: int(hostUID),
			GID: int(hostGID),
		}

		// buildkit parity: change uid and gid if specified, otherwise keep `0`
		err = idtools.MkdirAllAndChownNew(thisCacheRoot, os.FileMode(mode), idPair)
		if err != nil {
			return newMount, "", "", "", nil, fmt.Errorf("unable to change uid,gid of cache directory: %w", err)
		}
	}

	// path should be /mountPoint/specified path
	evaluated, err := copier.EvalContext(ctx, thisCacheRoot, thisCacheRoot+string(filepath.Separator)+newMount.Source, copier.EvalOptions{})
	if err != nil {
		return newMount, "", "", "", nil, err
	}
	newMount.Source = evaluated

	if sharing == "locked" {
		lockfile, err := cacheLockFile(cacheParent, lockID, BuildahCacheLockfile)
		if err != nil {
			return newMount, "", "", "", nil, fmt.Errorf("unable to acquire lock when sharing mode is locked: %w", err)
		}

		// will be unlocked after the RUN step is executed
		lockfile.Lock()
		targetLock = lockfile
	}

	// buildkit parity: default sharing should be shared
	// unless specified
	if !setShared {
		newMount.Options = append(newMount.Options, "shared")
	}

	// buildkit parity: cache must be writable unless `ro` or `readonly` is configured explicitly
	if setReadOnly == "" {
		newMount.Options = append(newMount.Options, "rw")
	}

	newMount.Options = append(newMount.Options, "bind")

	opts, err := parse.ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, "", "", "", nil, err
	}
	newMount.Options = opts

	var intermediateMount string
	if newMount.Source != thisCacheRoot {
		rel, err := filepath.Rel(thisCacheRoot, newMount.Source)
		if err != nil {
			return newMount, "", "", "", nil, fmt.Errorf("computing pathname of cache subdirectory: %w", err)
		}
		if rel != "." && rel != "/" {
			mnt, err := bindFromChroot(thisCacheRoot, rel, tmpDir)
			if err != nil {
				return newMount, "", "", "", nil, fmt.Errorf("sanitizing cache subdirectory %q: %w", newMount.Source, err)
			}
			logrus.Debugf("bind-mounted %q under %q to %q", rel, thisCacheRoot, mnt)
			intermediateMount = mnt
			newMount.Source = intermediateMount
		}
	}

	overlayDir := ""
	if needToOverlay {
		if newMount, overlayDir, err = convertToOverlay(newMount, store, mountLabel, tmpDir, 0, 0); err != nil {
			return newMount, "", "", "", nil, err
		}
	}

	succeeded = true
	return newMount, mountedImage, intermediateMount, overlayDir, targetLock, nil
}

// cacheLockFile returns the lock file with the given name for lockID, creating
// the directory which holds it if necessary.
func cacheLockFile(cacheParent, lockID, lockName string) (*lockfile.LockFile, error) {
	lockFilesDir := filepath.Join(cacheParent, BuildahCacheLockfileDir, lockID)
	if err := os.MkdirAll(lockFilesDir, os.FileMode(0o755)); err != nil {
		return nil, fmt.Errorf("unable to create build cache lock file directory: %w", err)
	}
	return lockfile.GetLockFile(filepath.Join(lockFilesDir, lockName))
}

// lockPrivateCacheDir locks a directory which no other build is using for the
// cache named dirID: that directory, or the first free one of "<dirID>-1",
// "<dirID>-2" and so on.  The pool grows to the number of builds which use the
// cache at the same time.
//
// Returns the directory's name and its lock, for the caller to release when
// it's finished.
func lockPrivateCacheDir(cacheParent, dirID string) (string, *lockfile.LockFile, error) {
	// Builds pick from this cache's pool one at a time, so we take the lowest
	// free directory, and the pool only grows when they are all busy.
	poolLock, err := cacheLockFile(cacheParent, dirID, buildahCachePoolLockfile)
	if err != nil {
		return "", nil, fmt.Errorf("unable to lock the cache's pool of directories: %w", err)
	}
	poolLock.Lock()
	defer poolLock.Unlock()

	// only so many builds can use this cache at once, so a name is always free
	for i := 0; ; i++ {
		candidate, added := dirID, false
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", dirID, i)
			// numbered directories are only for sharing=private, and get
			// added under the pool lock, so if this one isn't there yet,
			// we're the ones adding it
			_, err := os.Stat(filepath.Join(cacheParent, BuildahCacheLockfileDir, candidate))
			added = errors.Is(err, os.ErrNotExist)
		}
		lock, err := cacheLockFile(cacheParent, candidate, BuildahCacheLockfile)
		if err != nil {
			return "", nil, fmt.Errorf("unable to acquire lock when sharing mode is private: %w", err)
		}
		lockErr := lock.TryLock()
		if lockErr == nil {
			return candidate, lock, nil
		}
		if added {
			return "", nil, fmt.Errorf("unable to lock new private cache directory %q: %w", candidate, lockErr)
		}
		logrus.Debugf("cache directory %q is in use, trying another one: %v", candidate, lockErr)
	}
}

func getVolumeMounts(volumes []string) (map[string]specs.Mount, error) {
	finalVolumeMounts := make(map[string]specs.Mount)

	for _, volume := range volumes {
		volumeMount, err := internalParse.Volume(volume)
		if err != nil {
			return nil, err
		}
		if _, ok := finalVolumeMounts[volumeMount.Destination]; ok {
			return nil, fmt.Errorf("%v: %w", volumeMount.Destination, errDuplicateDest)
		}
		finalVolumeMounts[volumeMount.Destination] = volumeMount
	}
	return finalVolumeMounts, nil
}

// UnlockLockArray is a helper for cleaning up after GetVolumes and the like.
func UnlockLockArray(locks []*lockfile.LockFile) {
	for _, lock := range locks {
		lock.Unlock()
	}
}

// GetVolumes gets the volumes from --volume and --mount flags.
//
// Returns a slice of Mounts to add to the runtime spec's list of mounts, the
// IDs of any images we mounted, a slice of bind-mounted paths, a slice of
// overlay directories and a slice of locks that we acquired, or an error.
//
// The caller is expected to, after the command which uses the mounts and
// volumes exits, clean up the overlay directories, unmount and remove the
// mountpoints for the bind-mounted paths, unmount any images we mounted, and
// release the locks we returned (either using UnlockLockArray() or by
// iterating over them and unlocking them).
func GetVolumes(ctx context.Context, sys *types.SystemContext, store storage.Store, mountLabel string, volumes []string, mounts []string, contextDir string, idMaps define.IDMappingOptions, workDir, tmpDir string) ([]specs.Mount, []string, []string, []string, []*lockfile.LockFile, error) {
	select {
	case <-ctx.Done():
		return nil, nil, nil, nil, nil, ctx.Err()
	default:
	}

	unifiedMounts, mountedImages, intermediateMounts, overlayMounts, targetLocks, err := getMounts(ctx, sys, store, mountLabel, mounts, contextDir, idMaps.UIDMap, idMaps.GIDMap, workDir, tmpDir)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			for _, overlayMount := range overlayMounts {
				if err := overlay.RemoveTemp(overlayMount); err != nil {
					logrus.Debugf("unmounting overlay at %q: %v", overlayMount, err)
				}
			}
			for _, intermediateMount := range intermediateMounts {
				if err := mount.Unmount(intermediateMount); err != nil {
					logrus.Debugf("unmounting intermediate mount point %q: %v", intermediateMount, err)
				}
				if err := os.Remove(intermediateMount); err != nil {
					logrus.Debugf("removing should-be-empty directory %q: %v", intermediateMount, err)
				}
			}
			for _, image := range mountedImages {
				if _, err := store.UnmountImage(image, false); err != nil {
					logrus.Debugf("unmounting image %q: %v", image, err)
				}
			}
			UnlockLockArray(targetLocks)
		}
	}()
	volumeMounts, err := getVolumeMounts(volumes)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for dest, mount := range volumeMounts {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", dest, errDuplicateDest)
		}
		unifiedMounts[dest] = mount
	}

	finalMounts := make([]specs.Mount, 0, len(unifiedMounts))
	for _, mount := range unifiedMounts {
		finalMounts = append(finalMounts, mount)
	}
	succeeded = true
	return finalMounts, mountedImages, intermediateMounts, overlayMounts, targetLocks, nil
}

// getMounts takes user-provided inputs from the --mount flag and returns a
// slice of OCI spec mounts, a slice of mounted image IDs, a slice of other
// mount locations, a slice of overlay mounts, and a slice of locks, or an
// error.
//
//	buildah run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
//	buildah run --mount type=cache,target=/var/cache ...
//	buildah run --mount type=tmpfs,target=/dev/shm ...
//
// The caller is expected to, after the command which uses the mounts exits,
// unmount the overlay filesystems (if we mounted any), unmount the other
// mounted filesystems and remove their mountpoints (if we provided any paths
// to mountpoints), unmount any mounted images (if we provided the IDs of any),
// and then unlock the locks we returned (either using UnlockLockArray() or by
// iterating over them and unlocking them).
func getMounts(ctx context.Context, sys *types.SystemContext, store storage.Store, mountLabel string, mounts []string, contextDir string, uidmap, gidmap []specs.LinuxIDMapping, workDir, tmpDir string) (map[string]specs.Mount, []string, []string, []string, []*lockfile.LockFile, error) {
	select {
	case <-ctx.Done():
		return nil, nil, nil, nil, nil, ctx.Err()
	default:
	}

	// If `type` is not set default to "bind"
	mountType := define.TypeBind
	finalMounts := make(map[string]specs.Mount, len(mounts))
	mountedImages := make([]string, 0, len(mounts))
	intermediateMounts := make([]string, 0, len(mounts))
	overlayMounts := make([]string, 0, len(mounts))
	targetLocks := make([]*lockfile.LockFile, 0, len(mounts))
	succeeded := false
	defer func() {
		if !succeeded {
			for _, overlayDir := range overlayMounts {
				if err := overlay.RemoveTemp(overlayDir); err != nil {
					logrus.Debugf("unmounting overlay mount at %q: %v", overlayDir, err)
				}
			}
			for _, intermediateMount := range intermediateMounts {
				if err := mount.Unmount(intermediateMount); err != nil {
					logrus.Debugf("unmounting intermediate mount point %q: %v", intermediateMount, err)
				}
				if err := os.Remove(intermediateMount); err != nil {
					logrus.Debugf("removing should-be-empty directory %q: %v", intermediateMount, err)
				}
			}
			for _, image := range mountedImages {
				if _, err := store.UnmountImage(image, false); err != nil {
					logrus.Debugf("unmounting image %q: %v", image, err)
				}
			}
			UnlockLockArray(targetLocks)
		}
	}()

	errInvalidSyntax := errors.New("incorrect mount format: should be --mount type=<bind|tmpfs>,[src=<host-dir>,]target=<ctr-dir>[,options]")

	for _, mount := range mounts {
		tokens := strings.Split(mount, ",")
		if len(tokens) < 2 {
			return nil, nil, nil, nil, nil, fmt.Errorf("%q: %w", mount, errInvalidSyntax)
		}
		for _, field := range tokens {
			if strings.HasPrefix(field, "type=") {
				kv := strings.Split(field, "=")
				if len(kv) != 2 {
					return nil, nil, nil, nil, nil, fmt.Errorf("%q: %w", mount, errInvalidSyntax)
				}
				mountType = kv[1]
			}
		}
		switch mountType {
		case define.TypeBind:
			mount, image, intermediateMount, overlayMount, err := GetBindMount(ctx, sys, tokens, contextDir, store, mountLabel, nil, workDir, tmpDir)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			if image != "" {
				mountedImages = append(mountedImages, image)
			}
			if intermediateMount != "" {
				intermediateMounts = append(intermediateMounts, intermediateMount)
			}
			if overlayMount != "" {
				overlayMounts = append(overlayMounts, overlayMount)
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case TypeCache:
			mount, image, intermediateMount, overlayMount, tl, err := GetCacheMount(ctx, sys, tokens, store, "", nil, uidmap, gidmap, workDir, tmpDir)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			if image != "" {
				mountedImages = append(mountedImages, image)
			}
			if intermediateMount != "" {
				intermediateMounts = append(intermediateMounts, intermediateMount)
			}
			if overlayMount != "" {
				overlayMounts = append(overlayMounts, overlayMount)
			}
			if tl != nil {
				targetLocks = append(targetLocks, tl)
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		case TypeTmpfs:
			mount, err := GetTmpfsMount(tokens, workDir)
			if err != nil {
				return nil, nil, nil, nil, nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, nil, nil, nil, nil, fmt.Errorf("%v: %w", mount.Destination, errDuplicateDest)
			}
			finalMounts[mount.Destination] = mount
		default:
			return nil, nil, nil, nil, nil, fmt.Errorf("invalid filesystem type %q", mountType)
		}
	}

	succeeded = true
	return finalMounts, mountedImages, intermediateMounts, overlayMounts, targetLocks, nil
}

// GetTmpfsMount parses a single tmpfs mount entry from the --mount flag
func GetTmpfsMount(args []string, workDir string) (specs.Mount, error) {
	newMount := specs.Mount{
		Type:   TypeTmpfs,
		Source: TypeTmpfs,
	}

	setDest := false

	for _, val := range args {
		argName, argValue, hasArgValue := strings.Cut(val, "=")
		switch argName {
		case "type":
			// This is already processed, and should be "tmpfs"
			continue
		case "nosuid", "nodev", "noexec":
			if hasArgValue {
				return newMount, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, argName)
		case "ro", "readonly":
			if hasArgValue {
				return newMount, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			newMount.Options = append(newMount.Options, "ro")
		case "tmpcopyup":
			if hasArgValue {
				return newMount, fmt.Errorf("%v: %w", argName, errBadOptionNoArg)
			}
			// the path that is shadowed by the tmpfs mount is recursively copied up to the tmpfs itself.
			newMount.Options = append(newMount.Options, argName)
		case "tmpfs-mode":
			if !hasArgValue || argValue == "" {
				return newMount, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("mode=%s", argValue))
		case "tmpfs-size":
			if !hasArgValue || argValue == "" {
				return newMount, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("size=%s", argValue))
		case "target", "dst", "destination":
			if !hasArgValue || argValue == "" {
				return newMount, fmt.Errorf("%v: %w", argName, errBadOptionArg)
			}
			targetPath := argValue
			if !path.IsAbs(targetPath) {
				targetPath = filepath.Join(workDir, targetPath)
			}
			if err := parse.ValidateVolumeCtrDir(targetPath); err != nil {
				return newMount, err
			}
			newMount.Destination = targetPath
			setDest = true
		default:
			return newMount, fmt.Errorf("%v: %w", argName, errBadMntOption)
		}
	}

	if !setDest {
		return newMount, errBadVolDest
	}

	return newMount, nil
}
