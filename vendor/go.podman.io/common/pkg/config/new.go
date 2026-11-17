package config

import (
	"fmt"
	"sync"

	"go.podman.io/storage/pkg/configfile"
	"go.podman.io/storage/pkg/unshare"
)

var (
	cachedConfigError error
	cachedConfigMutex sync.Mutex
	cachedConfig      *Config
)

const (
	containersConfEnv         = "CONTAINERS_CONF"
	containersConfOverrideEnv = containersConfEnv + "_OVERRIDE"
)

// Options to use when loading a Config via New().
type Options struct {
	// Attempt to load the following config modules.
	Modules []string

	// Set the loaded config as the default one which can later on be
	// accessed via Default().
	SetDefault bool
}

func defaultConfigFileOpts() *configfile.File {
	return &configfile.File{
		Name:            "containers",
		Extension:       "conf",
		EnvironmentName: containersConfEnv,
		UserId:          unshare.GetRootlessUID(),
	}
}

// New returns a Config as described in the containers.conf(5) man page.
func New(options *Options) (*Config, error) {
	if options == nil {
		options = &Options{}
	} else if options.SetDefault {
		cachedConfigMutex.Lock()
		defer cachedConfigMutex.Unlock()
	}
	return newLocked(options, defaultConfigFileOpts())
}

// Default returns the default container config.  If no default config has been
// set yet, a new config will be loaded by New() and set as the default one.
// All callers are expected to use the returned Config read only.  Changing
// data may impact other call sites.
func Default() (*Config, error) {
	cachedConfigMutex.Lock()
	defer cachedConfigMutex.Unlock()
	if cachedConfig != nil || cachedConfigError != nil {
		return cachedConfig, cachedConfigError
	}
	cachedConfig, cachedConfigError = newLocked(&Options{SetDefault: true}, defaultConfigFileOpts())
	return cachedConfig, cachedConfigError
}

// A helper function for New() expecting the caller to hold the
// cachedConfigMutex if options.SetDefault is set..
func newLocked(options *Options, file *configfile.File) (*Config, error) {
	// Start with the built-in defaults
	config, err := defaultConfig()
	if err != nil {
		return nil, err
	}

	file.Modules = options.Modules

	err = configfile.ParseTOML(config, file)
	if err != nil {
		return nil, fmt.Errorf("parsing containers.conf: %w", err)
	}
	config.loadedModules = file.Modules

	config.addCAPPrefix()

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if err := config.setupEnv(); err != nil {
		return nil, err
	}

	if options.SetDefault {
		cachedConfig = config
		cachedConfigError = nil
	}

	return config, nil
}
