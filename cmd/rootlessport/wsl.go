//go:build linux

package main

import (
	"net"
	"strings"

	rkport "github.com/rootless-containers/rootlesskit/v2/pkg/port"
	"go.podman.io/common/pkg/machine"
)

// WSL machines do not relay ipv4 traffic to dual-stack ports, simulate instead
func splitDualStackSpecIfWsl(spec rkport.Spec) []rkport.Spec {
	specs := []rkport.Spec{spec}
	protocol := spec.Proto
	if machine.HostType() != machine.Wsl || strings.HasSuffix(protocol, "4") || strings.HasSuffix(protocol, "6") {
		return specs
	}

	ip := net.ParseIP(spec.ParentIP)
	splitLoopback := ip.IsLoopback() && ip.To4() == nil
	// Map ::1 and 0.0.0.0/:: to ipv4 + ipv6 to simulate dual-stack
	if ip.IsUnspecified() || splitLoopback {
		specs = append(specs, spec)
		specs[0].Proto = protocol + "4"
		specs[1].Proto = protocol + "6"
		if splitLoopback {
			// Hacky, but we will only have one ipv4 loopback with WSL config
			specs[0].ParentIP = "127.0.0.1"
		}
		if ip.IsUnspecified() {
			specs[0].ParentIP = "0.0.0.0"
			specs[1].ParentIP = "::"
		}
	}

	return specs
}
