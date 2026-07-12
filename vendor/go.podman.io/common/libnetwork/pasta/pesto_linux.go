// Pesto client for dynamic port forwarding on a running pasta instance.
//
// Pesto updates pasta's forwarding table via a UNIX domain socket (-c).
// Used by rootless bridge networking: pesto incrementally adds or deletes
// port forwarding rules for individual containers.
//
// Passt only forwards traffic from the host into the rootless netns.
// Netavark handles the final DNAT to the container IP:ContainerPort
// inside the netns. Each mapping uses HostPort as both source and
// destination so traffic arrives at the port netavark expects.
//
// When no HostIP is specified, pesto binds both IPv4 (0.0.0.0) and
// IPv6 ([::]) so dual-stack networks work out of the box.
//
// Limitations:
//   - TCP and UDP only (SCTP is silently skipped)

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
func PestoAddPorts(conf *config.Config, socketPath string, ports []types.PortMapping) error {
	if socketPath == "" {
		return errors.New("pesto control socket not available")
	}
	logrus.Debugf("pesto: adding %d port mappings", len(ports))
	return pestoModifyPorts(conf, socketPath, ports, "--add")
}

// PestoDeletePorts removes port forwarding rules from the running pasta
// instance via -D/--delete.
func PestoDeletePorts(conf *config.Config, socketPath string, ports []types.PortMapping) error {
	if socketPath == "" {
		return nil
	}
	logrus.Debugf("pesto: deleting %d port mappings", len(ports))
	return pestoModifyPorts(conf, socketPath, ports, "--delete")
}

func pestoModifyPorts(conf *config.Config, socketPath string, ports []types.PortMapping, mode string) error {
	pestoPath, err := conf.FindHelperBinary(PestoBinaryName, true)
	if err != nil {
		return fmt.Errorf("could not find pesto binary: %w", err)
	}

	pestoArgs, err := portMappingsToPestoArgs(ports)
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

// portMappingsToPestoArgs converts PortMappings into pesto CLI arguments.
//
// When HostIP is set, a single binding is created (e.g. "-t 127.0.0.1/8080").
// When HostIP is empty, both IPv4 and IPv6 bindings are created so that
// dual-stack networks work: "-t 0.0.0.0/8080 -t [::]/8080".
func portMappingsToPestoArgs(ports []types.PortMapping) ([]string, error) {
	var args []string

	for _, p := range ports {
		var addrs []string
		switch {
		case p.HostIP == "":
			addrs = []string{"0.0.0.0/", "[::]/"}
		case strings.Contains(p.HostIP, ":"):
			addrs = []string{"[" + p.HostIP + "]/"}
		default:
			addrs = []string{p.HostIP + "/"}
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

			for _, addr := range addrs {
				var arg string
				if portRange == 1 {
					arg = fmt.Sprintf("%s%d", addr, p.HostPort)
				} else {
					arg = fmt.Sprintf("%s%d-%d", addr, p.HostPort, p.HostPort+portRange-1)
				}
				args = append(args, flag, arg)
			}
		}
	}
	return args, nil
}
