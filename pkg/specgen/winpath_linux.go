package specgen

import (
	"go.podman.io/common/pkg/machine"
	"go.podman.io/storage/pkg/fileutils"
)

func shouldResolveWinPaths() bool {
	hostType := machine.HostType()
	return hostType == machine.Wsl || hostType == machine.HyperV
}

func shouldResolveUnixWinVariant(path string) bool {
	return fileutils.Exists(path) != nil
}

func resolveRelativeOnWindows(path string) string {
	return path
}

func winPathExists(_ string) bool {
	return false
}
