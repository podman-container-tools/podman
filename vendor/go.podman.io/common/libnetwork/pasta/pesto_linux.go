// Pesto client for dynamic port forwarding on a running pasta instance.
//
// Pesto updates pasta's forwarding table via a UNIX domain socket (-c).
// Used by rootless bridge networking: pesto incrementally adds or deletes
// port forwarding rules for individual containers.
//
// Each mapping specifies both the host binding and container target
// address, so pasta forwards traffic directly to the correct
// container IP:ContainerPort.
//
// When no HostIP is specified, pesto binds both IPv4 (0.0.0.0) and
// IPv6 ([::]) so dual-stack networks work out of the box.
//
// Limitations:
//   - TCP and UDP only (unsupported protocols return an error)

package pasta

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
)

const PestoBinaryName = "pesto"

// PestoAddPorts adds port forwarding rules to the running pasta instance
// via -A/--add. Idempotent: adding already-active ports is a no-op.
// containerIPv4 and containerIPv6 are the container's addresses inside the
// network namespace; they are embedded in the target side of each mapping.
func PestoAddPorts(conf *config.Config, socketPath string, ports []types.PortMapping, containerIPv4, containerIPv6 string) error {
	if socketPath == "" {
		return errors.New("pesto control socket not available")
	}
	logrus.Debugf("pesto: adding %d port mappings", len(ports))
	return pestoModifyPorts(conf, socketPath, ports, "--add", containerIPv4, containerIPv6)
}

// PestoDeletePorts removes port forwarding rules from the running pasta
// instance via -D/--delete.
// containerIPv4 and containerIPv6 are the container's addresses inside the
// network namespace; they are embedded in the target side of each mapping.
func PestoDeletePorts(conf *config.Config, socketPath string, ports []types.PortMapping, containerIPv4, containerIPv6 string) error {
	if socketPath == "" {
		return nil
	}
	logrus.Debugf("pesto: deleting %d port mappings", len(ports))
	return pestoModifyPorts(conf, socketPath, ports, "--delete", containerIPv4, containerIPv6)
}

func pestoModifyPorts(conf *config.Config, socketPath string, ports []types.PortMapping, mode, containerIPv4, containerIPv6 string) error {
	pestoPath, err := conf.FindHelperBinary(PestoBinaryName, true)
	if err != nil {
		return fmt.Errorf("could not find pesto binary: %w", err)
	}

	pestoArgs, err := portMappingsToPestoArgs(ports, containerIPv4, containerIPv6)
	if err != nil {
		return err
	}
	args := make([]string, 0, len(pestoArgs)+2) // +2 for mode and socket path
	args = append(args, mode)
	args = append(args, pestoArgs...)
	args = append(args, socketPath)

	logrus.Debugf("pesto arguments: %s", strings.Join(args, " "))

	out, err := exec.Command(pestoPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pesto failed: %w\noutput: %s", err, string(out))
	}
	if len(out) > 0 {
		logrus.Debugf("pesto output: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// portMappingsToPestoArgs converts PortMappings into pesto CLI arguments
// using the target mapping syntax: -t hostIP/hostPort:containerIP/containerPort.
//
// IPv6 host addresses are bracketed (e.g. [::]) to disambiguate colons
// from the mapping ":" separator; container addresses are never bracketed
// because they appear after the ":" where there is no ambiguity.
//
// When HostIP is empty, dual-stack bindings are created using 0.0.0.0 with
// containerIPv4, and (if containerIPv6 is non-empty) [::] with containerIPv6.
// When HostIP is set, the address-family-matched container IP is used.
func portMappingsToPestoArgs(ports []types.PortMapping, containerIPv4, containerIPv6 string) ([]string, error) {
	type addrPair struct {
		host, container string
	}

	var args []string

	for _, p := range ports {
		var pairs []addrPair

		switch {
		case p.HostIP == "":
			if containerIPv4 != "" {
				pairs = append(pairs, addrPair{"0.0.0.0", containerIPv4})
			}
			if containerIPv6 != "" {
				pairs = append(pairs, addrPair{"[::]", containerIPv6})
			}
		case strings.Contains(p.HostIP, ":"):
			if containerIPv6 != "" {
				pairs = append(pairs, addrPair{"[" + p.HostIP + "]", containerIPv6})
			}
		default:
			if containerIPv4 != "" {
				pairs = append(pairs, addrPair{p.HostIP, containerIPv4})
			}
		}

		for protocol := range strings.SplitSeq(p.Protocol, ",") {
			var flag string
			switch protocol {
			case "tcp":
				flag = "-t"
			case "udp":
				flag = "-u"
			default:
				return nil, fmt.Errorf("pesto: unsupported protocol %s", protocol)
			}

			portRange := p.Range
			if portRange == 0 {
				portRange = 1
			}

			for _, pair := range pairs {
				var arg string
				if portRange == 1 {
					arg = fmt.Sprintf("%s/%d:%s/%d", pair.host, p.HostPort, pair.container, p.ContainerPort)
				} else {
					arg = fmt.Sprintf("%s/%d-%d:%s/%d-%d",
						pair.host, p.HostPort, p.HostPort+portRange-1,
						pair.container, p.ContainerPort, p.ContainerPort+portRange-1)
				}
				args = append(args, flag, arg)
			}
		}
	}
	return args, nil
}
