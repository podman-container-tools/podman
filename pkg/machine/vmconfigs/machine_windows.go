package vmconfigs

import (
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
)

func getPipe(name string) *define.VMFile {
	pipeName := env.WithPodmanPrefix(name)
	return &define.VMFile{Path: `\\.\pipe\` + pipeName}
}
