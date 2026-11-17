//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package vmconfigs

import (
	"go.podman.io/podman/v6/pkg/machine/define"
)

func getPipe(_ string) *define.VMFile {
	return nil
}
