//go:build !windows

package configfile

import (
	"os"
	"path/filepath"

	"go.podman.io/storage/pkg/unshare"
)

// UserConfigPath returns the path to the users local config that is
// not shared with other users. It uses $XDG_CONFIG_HOME/containers...
// if set or $HOME/.config/containers... if not.
func UserConfigPath() (string, error) {
	if configHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok {
		return filepath.Join(configHome, _configPathName), nil
	}
	home, err := unshare.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", _configPathName), nil
}
