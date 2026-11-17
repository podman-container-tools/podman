package vmconfigs

import (
	"fmt"

	"go.podman.io/podman/v6/pkg/machine/define"
)

func gvProxySocket(name string, machineRuntimeDir *define.VMFile) (*define.VMFile, error) {
	socketName := fmt.Sprintf("%s-gvproxy.sock", name)
	return machineRuntimeDir.AppendToNewVMFile(socketName, &socketName)
}

func readySocket(name string, machineRuntimeDir *define.VMFile) (*define.VMFile, error) {
	socketName := name + ".sock"
	return machineRuntimeDir.AppendToNewVMFile(socketName, &socketName)
}

func apiSocket(name string, socketDir *define.VMFile) (*define.VMFile, error) {
	socketName := name + "-api.sock"
	return socketDir.AppendToNewVMFile(socketName, &socketName)
}
