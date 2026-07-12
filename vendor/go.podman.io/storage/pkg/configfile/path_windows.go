package configfile

import (
	"os"
	"path/filepath"
)

const (
	// builtinSystemConfigPath is the location for the default config files shipped by the distro/vendor.
	// On windows there is no /usr equivalent so leave it empty.
	builtinSystemConfigPath = ""
)

func getAdminOverrideConfigPath() string {
	if env, ok := os.LookupEnv("ProgramData"); ok {
		return filepath.Join(env, _configPathName)
	}
	return ""
}

// UserConfigPath returns the path to the users local config that is
// not shared with other users. It uses $APPDATA/containers...
func UserConfigPath() (string, error) {
	if env, ok := os.LookupEnv("APPDATA"); ok {
		return filepath.Join(env, _configPathName), nil
	}
	return "", nil
}
