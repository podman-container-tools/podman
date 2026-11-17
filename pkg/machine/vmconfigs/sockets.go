//go:build !darwin

package vmconfigs

import (
	"fmt"

	"go.podman.io/podman/v6/pkg/machine/define"
)

func gvProxySocket(name string, machineRuntimeDir *define.VMFile) (*define.VMFile, error) {
	return machineRuntimeDir.AppendToNewVMFile(fmt.Sprintf("%s-gvproxy.sock", name), nil)
}

func readySocket(name string, machineRuntimeDir *define.VMFile) (*define.VMFile, error) {
	return machineRuntimeDir.AppendToNewVMFile(name+".sock", nil)
}

func apiSocket(name string, socketDir *define.VMFile) (*define.VMFile, error) {
	return socketDir.AppendToNewVMFile(name+"-api.sock", nil)
}
