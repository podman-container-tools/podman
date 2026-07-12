//go:build freebsd || netbsd

package types

const (
	// these are default path for run and graph root for rootful users
	// for rootless path is constructed via getRootlessStorageOpts
	defaultRunRoot   string = "/var/run/containers/storage"
	defaultGraphRoot string = "/var/db/containers/storage"
)
