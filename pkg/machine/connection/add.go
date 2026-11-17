//go:build amd64 || arm64

package connection

import (
	"fmt"
	"strconv"

	"go.podman.io/podman/v6/pkg/machine/define"
)

// AddSSHConnectionsToPodmanSocket adds SSH connections to the podman socket if
// no ignition path is provided
func AddSSHConnectionsToPodmanSocket(uid, port int, identityPath, name, remoteUsername string, opts define.InitOptions) error {
	if len(opts.IgnitionPath) > 0 {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
		return nil
	}

	cons := createConnections(name, uid, port, remoteUsername)

	// The first connection defined when connections is empty will become the default
	// regardless of IsDefault, so order according to rootful
	if opts.Rootful {
		cons[0], cons[1] = cons[1], cons[0]
	}

	return addConnection(cons, identityPath, opts.IsDefault)
}

func createConnections(name string, uid, port int, remoteUsername string) []connection {
	uri := makeSSHURL(LocalhostIP, fmt.Sprintf("/run/user/%d/podman/podman.sock", uid), strconv.Itoa(port), remoteUsername)
	uriRoot := makeSSHURL(LocalhostIP, "/run/podman/podman.sock", strconv.Itoa(port), "root")

	return []connection{
		{
			name: name,
			uri:  uri,
		},
		{
			name: name + "-root",
			uri:  uriRoot,
		},
	}
}
