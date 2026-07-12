package configfile

const (
	// builtinSystemConfigPath is the location for the default config files shipped by the distro/vendor.
	builtinSystemConfigPath = "/usr/local/share/" + _configPathName

	// builtinAdminOverrideConfigPath is the location for admin local override config files.
	builtinAdminOverrideConfigPath = "/usr/local/etc/" + _configPathName
)

func getAdminOverrideConfigPath() string {
	return builtinAdminOverrideConfigPath
}
