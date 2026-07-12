//go:build !remote

// Pesto integration for rootless bridge network port forwarding.
//
// A shared pasta instance in the rootless netns (-c pasta.sock) handles
// host-side port forwarding. On container start/stop, pesto incrementally
// adds or deletes port forwarding rules for that container. Pasta forwards
// via kernel splice (localhost) or TAP (external), preserving source IPs.
// The container sees the real client's address instead of a proxy or bridge
// gateway address.
//
// Container start:
// - netavark sets up bridge + DNAT
// - pesto --add: adds this container's ports to pasta
//
// Container stop:
// - pesto --delete: removes this container's ports from pasta
// - netavark tears down bridge/DNAT

package libpod

import (
	"go.podman.io/common/libnetwork/pasta"
)

func (r *Runtime) pestoSocketPath() string {
	return r.network.PestoSocketPath()
}

// setupRootlessPortMappingViaPesto adds this container's port forwarding
// rules to the shared pasta instance.
func (r *Runtime) setupRootlessPortMappingViaPesto(ctr *Container) error {
	ports := ctr.convertPortMappings()
	if len(ports) == 0 {
		return nil
	}
	return pasta.PestoAddPorts(r.config, r.pestoSocketPath(), ports)
}

// teardownRootlessPortMappingViaPesto removes this container's port
// forwarding rules from the shared pasta instance.
func (r *Runtime) teardownRootlessPortMappingViaPesto(ctr *Container) error {
	ports := ctr.convertPortMappings()
	if len(ports) == 0 {
		return nil
	}
	return pasta.PestoDeletePorts(r.config, r.pestoSocketPath(), ports)
}
