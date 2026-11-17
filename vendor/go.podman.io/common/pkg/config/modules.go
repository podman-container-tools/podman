package config

import "go.podman.io/storage/pkg/configfile"

// LoadedModules returns absolute paths to loaded containers.conf modules.
func (c *Config) LoadedModules() []string {
	// Required for conmon's callback to Podman's cleanup.
	// Absolute paths make loading the modules a bit faster.
	return c.loadedModules
}

// ModuleDirectories return the directories to load modules from:
// 1) XDG_CONFIG_HOME/HOME if rootless
// 2) /etc/
// 3) /usr/share.
func ModuleDirectories() ([]string, error) { // Public API for shell completions in Podman
	conf := defaultConfigFileOpts()
	// API needs us to pass an non empty string to trigger module path resolution
	conf.Modules = []string{""}
	paths, err := configfile.GetSearchPaths(conf)
	return paths.ModuleDirectories, err
}
