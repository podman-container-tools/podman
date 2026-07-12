//go:build freebsd || netbsd || openbsd

package config

var defaultHelperBinariesDir = []string{
	"/usr/local/bin",
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
	"/usr/local/libexec/podman",
	"/usr/local/lib/podman",
}
