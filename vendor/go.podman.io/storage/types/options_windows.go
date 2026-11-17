package types

const (
	// these are default path for run and graph root for rootful users
	// for rootless path is constructed via getRootlessStorageOpts
	defaultRunRoot   string = "/run/containers/storage"
	defaultGraphRoot string = "/var/lib/containers/storage"
)
