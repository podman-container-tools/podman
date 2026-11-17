package types

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	cfg "go.podman.io/storage/pkg/config"
	"go.podman.io/storage/pkg/configfile"
	"go.podman.io/storage/pkg/homedir"
	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/unshare"
)

// TOML-friendly explicit tables used for conversions.
type TomlConfig struct {
	Storage struct {
		Driver              string            `toml:"driver,omitempty"`
		DriverPriority      configfile.Slice  `toml:"driver_priority,omitempty"`
		RunRoot             string            `toml:"runroot,omitempty"`
		ImageStore          string            `toml:"imagestore,omitempty"`
		GraphRoot           string            `toml:"graphroot,omitempty"`
		RootlessStoragePath string            `toml:"rootless_storage_path,omitempty"`
		TransientStore      bool              `toml:"transient_store,omitempty"`
		Options             cfg.OptionsConfig `toml:"options,omitempty"`
	} `toml:"storage"`
}

const (
	overlayDriver  = "overlay"
	overlay2       = "overlay2"
	storageConfEnv = "CONTAINERS_STORAGE_CONF"
)

// usePerUserStorage returns whether the user private storage must be used.
// We cannot simply use the unshare.IsRootless() condition, because
// that checks only if the current process needs a user namespace to
// work and it would break cases where the process is already created
// in a user namespace (e.g. nested Podman/Buildah) and the desired
// behavior is to use system paths instead of user private paths.
func usePerUserStorage() bool {
	return unshare.IsRootless() && unshare.GetRootlessUID() != 0
}

// defaultStoreOptions is kept private so external callers can not reassign the value
var defaultStoreOptions = sync.OnceValues(func() (StoreOptions, error) {
	return LoadStoreOptions(LoadOptions{})
})

// DefaultStoreOptions is returning the default [StoreOptions] parsed with the storage.conf files.
// This function caches the result so multiple callers always will get the same result.
// In order to parse storage.conf files at a later point use [LoadStoreOptions].
func DefaultStoreOptions() (StoreOptions, error) {
	return defaultStoreOptions()
}

// StoreOptions is used for passing initialization options to GetStore(), for
// initializing a Store object and the underlying storage that it controls.
type StoreOptions struct {
	// RunRoot is the filesystem path under which we can store run-time
	// information, such as the locations of active mount points, that we
	// want to lose if the host is rebooted.
	RunRoot string `json:"runroot,omitempty"`
	// GraphRoot is the filesystem path under which we will store the
	// contents of layers, images, and containers.
	GraphRoot string `json:"root,omitempty"`
	// Image Store is the alternate location of image store if a location
	// separate from the container store is required.
	ImageStore string `json:"imagestore,omitempty"`
	// If the driver is not specified, the best suited driver will be picked
	// either from GraphDriverPriority, if specified, or from the platform
	// dependent priority list (in that order).
	GraphDriverName string `json:"driver,omitempty"`
	// GraphDriverPriority is a list of storage drivers that will be tried
	// to initialize the Store for a given RunRoot and GraphRoot unless a
	// GraphDriverName is set.
	// This list can be used to define a custom order in which the drivers
	// will be tried.
	GraphDriverPriority []string `json:"driver-priority,omitempty"`
	// GraphDriverOptions are driver-specific options.
	GraphDriverOptions []string `json:"driver-options,omitempty"`
	// UIDMap and GIDMap are used for setting up a container's root filesystem
	// for use inside of a user namespace where UID mapping is being used.
	UIDMap []idtools.IDMap `json:"uidmap,omitempty"`
	GIDMap []idtools.IDMap `json:"gidmap,omitempty"`
	// RootAutoNsUser is the user used to pick a subrange when automatically setting
	// a user namespace for the root user.
	RootAutoNsUser string `json:"root_auto_ns_user,omitempty"`
	// AutoNsMinSize is the minimum size for an automatic user namespace.
	AutoNsMinSize uint32 `json:"auto_userns_min_size,omitempty"`
	// AutoNsMaxSize is the maximum size for an automatic user namespace.
	AutoNsMaxSize uint32 `json:"auto_userns_max_size,omitempty"`
	// PullOptions specifies options to be handed to pull managers
	// This API is experimental and can be changed without bumping the major version number.
	PullOptions map[string]string `toml:"pull_options"`
	// DisableVolatile doesn't allow volatile mounts when it is set.
	DisableVolatile bool `json:"disable-volatile,omitempty"`
	// If transient, don't persist containers over boot (stores db in runroot)
	TransientStore bool `json:"transient_store,omitempty"`
}

// setDefaultRootlessStoreOptions sets the storage opts for containers running as non root
func setDefaultRootlessStoreOptions(opts *StoreOptions) error {
	dataDir, err := homedir.GetDataHome()
	if err != nil {
		return err
	}

	rootlessRuntime, err := homedir.GetRuntimeDir()
	if err != nil {
		return err
	}

	opts.GraphRoot = filepath.Join(dataDir, "containers", "storage")
	opts.RunRoot = filepath.Join(rootlessRuntime, "containers")

	return nil
}

type LoadOptions struct {
	// RootForImplicitAbsolutePaths is the path to an alternate root
	// If not "", prefixed to any absolute paths used by default in the package.
	// NOTE: This does NOT affect paths starting by $HOME or environment variables paths.
	RootForImplicitAbsolutePaths string
}

// LoadStoreOptions is returning the default [StoreOptions] parsed with the storage.conf files.
// [LoadOptions] can be used to supply an alternative root for the storage.conf files.
// On each call all files will be parsed again, to only get the correct system defaults
// and have them cached consider using [DefaultStoreOptions] instead.
func LoadStoreOptions(opts LoadOptions) (StoreOptions, error) {
	config := new(TomlConfig)

	rootlessUID := unshare.GetRootlessUID()
	err := configfile.ParseTOML(config, &configfile.File{
		Name:                         "storage",
		Extension:                    "conf",
		EnvironmentName:              storageConfEnv,
		UserId:                       rootlessUID,
		RootForImplicitAbsolutePaths: opts.RootForImplicitAbsolutePaths,
	})
	if err != nil {
		return StoreOptions{}, err
	}

	storeOptions := StoreOptions{
		GraphRoot: defaultGraphRoot,
		RunRoot:   defaultRunRoot,
	}
	if usePerUserStorage() {
		err := setDefaultRootlessStoreOptions(&storeOptions)
		if err != nil {
			return StoreOptions{}, err
		}
	}

	if config.Storage.Driver != "" {
		storeOptions.GraphDriverName = config.Storage.Driver
	}
	if val := os.Getenv("STORAGE_DRIVER"); val != "" {
		storeOptions.GraphDriverName = val
	}
	if storeOptions.GraphDriverName == overlay2 {
		logrus.Warnf("Switching default driver from overlay2 to the equivalent overlay driver")
		storeOptions.GraphDriverName = overlayDriver
	}
	storeOptions.GraphDriverPriority = config.Storage.DriverPriority.Values

	if config.Storage.ImageStore != "" {
		storeOptions.ImageStore = config.Storage.ImageStore
	}

	for _, s := range config.Storage.Options.AdditionalImageStores.Values {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "imagestore="+s)
	}
	for _, s := range config.Storage.Options.AdditionalLayerStores.Values {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "additionallayerstore="+s)
	}
	if config.Storage.Options.Size != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "size="+config.Storage.Options.Size)
	}
	if config.Storage.Options.MountProgram != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "mount_program="+config.Storage.Options.MountProgram)
	}
	if config.Storage.Options.SkipMountHome != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "skip_mount_home="+config.Storage.Options.SkipMountHome)
	}
	if config.Storage.Options.IgnoreChownErrors != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "ignore_chown_errors="+config.Storage.Options.IgnoreChownErrors)
	}
	if config.Storage.Options.ForceMask != 0 {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "force_mask="+strconv.FormatUint(uint64(config.Storage.Options.ForceMask), 8))
	}
	if config.Storage.Options.MountOpt != "" {
		storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, "mountopt="+config.Storage.Options.MountOpt)
	}
	storeOptions.RootAutoNsUser = config.Storage.Options.RootAutoUsernsUser
	if config.Storage.Options.AutoUsernsMinSize > 0 {
		storeOptions.AutoNsMinSize = config.Storage.Options.AutoUsernsMinSize
	}
	if config.Storage.Options.AutoUsernsMaxSize > 0 {
		storeOptions.AutoNsMaxSize = config.Storage.Options.AutoUsernsMaxSize
	}
	if config.Storage.Options.PullOptions != nil {
		storeOptions.PullOptions = config.Storage.Options.PullOptions
	}

	storeOptions.DisableVolatile = config.Storage.Options.DisableVolatile
	storeOptions.TransientStore = config.Storage.TransientStore

	storeOptions.GraphDriverOptions = append(storeOptions.GraphDriverOptions, cfg.GetGraphDriverOptions(config.Storage.Options)...)

	if opts, ok := os.LookupEnv("STORAGE_OPTS"); ok {
		storeOptions.GraphDriverOptions = strings.Split(opts, ",")
	}
	if len(storeOptions.GraphDriverOptions) == 1 && storeOptions.GraphDriverOptions[0] == "" {
		storeOptions.GraphDriverOptions = nil
	}

	if config.Storage.RunRoot != "" {
		runRoot, err := expandEnvPath(config.Storage.RunRoot, rootlessUID)
		if err != nil {
			return storeOptions, err
		}
		storeOptions.RunRoot = runRoot
	}
	if storeOptions.RunRoot == "" {
		return storeOptions, fmt.Errorf("runroot must be set")
	}

	// Parse this before graphroot which means any graphroot setting overwrites the
	// rootless_storage_path, so if the main config has both set only graphroot is used.
	if config.Storage.RootlessStoragePath != "" && usePerUserStorage() {
		storagePath, err := expandEnvPath(config.Storage.RootlessStoragePath, rootlessUID)
		if err != nil {
			return storeOptions, err
		}
		storeOptions.GraphRoot = storagePath
	}

	if config.Storage.GraphRoot != "" {
		if config.Storage.RootlessStoragePath != "" {
			logrus.Warn("Both rootless_storage_path and graphroot are set, using graphroot setting only, rootless_storage_path is deprecated and should be removed from the config")
		}
		graphRoot, err := expandEnvPath(config.Storage.GraphRoot, rootlessUID)
		if err != nil {
			return storeOptions, err
		}
		storeOptions.GraphRoot = graphRoot
	}

	if storeOptions.GraphRoot == "" {
		return storeOptions, fmt.Errorf("graphroot must be set")
	}

	if storeOptions.ImageStore != "" && storeOptions.ImageStore == storeOptions.GraphRoot {
		return storeOptions, fmt.Errorf("imagestore %s must either be not set or be different than graphroot", storeOptions.ImageStore)
	}

	return storeOptions, nil
}
