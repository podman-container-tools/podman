//go:build !windows && !freebsd

package configfile

const (
	// builtinSystemConfigPath is the location for the default config files shipped by the distro/vendor.
	builtinSystemConfigPath = "/usr/share/containers"

	// builtinAdminOverrideConfigPath is the location for admin local override config files.
	builtinAdminOverrideConfigPath = "/etc/containers"
)

func getAdminOverrideConfigPath() string {
	return builtinAdminOverrideConfigPath
}
