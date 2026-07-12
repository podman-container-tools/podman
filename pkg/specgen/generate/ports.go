//go:build !remote && (linux || freebsd)

package generate

import (
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/utils"
)

const (
	protoTCP  = "tcp"
	protoUDP  = "udp"
	protoSCTP = "sctp"
)

// joinTwoPortsToRangePortIfPossible will expect two ports the previous port one must have a lower or equal hostPort than the current port.
func joinTwoPortsToRangePortIfPossible(ports *[]types.PortMapping, allHostPorts, allContainerPorts, currentHostPorts *[65536]bool,
	previousPort *types.PortMapping, port types.PortMapping,
) (*types.PortMapping, error) {
	// no previous port just return the current one
	if previousPort == nil {
		return &port, nil
	}
	if previousPort.HostPort+previousPort.Range >= port.HostPort {
		// check if the port range matches the host and container ports
		portDiff := port.HostPort - previousPort.HostPort
		if portDiff == port.ContainerPort-previousPort.ContainerPort {
			// calc the new range use the old range and add the difference between the ports
			newRange := port.Range + portDiff
			// if the newRange is greater than the old range use it
			// this is important otherwise we would could lower the range
			if newRange > previousPort.Range {
				previousPort.Range = newRange
			}
			return previousPort, nil
		}
		// if both host port ranges overlap and the container port range did not match
		// we have to error because we cannot assign the same host port to more than one container port
		if previousPort.HostPort+previousPort.Range-1 > port.HostPort {
			return nil, fmt.Errorf("conflicting port mappings for host port %d (protocol %s)", port.HostPort, port.Protocol)
		}
	}
	// we could not join the ports so we append the old one to the list
	// and return the current port as previous port
	if err := addPortToUsedPorts(ports, allHostPorts, allContainerPorts, currentHostPorts, previousPort); err != nil {
		return nil, err
	}
	return &port, nil
}

// joinTwoContainerPortsToRangePortIfPossible will expect two ports with both no host port set,
//
//	the previous port one must have a lower or equal containerPort than the current port.
func joinTwoContainerPortsToRangePortIfPossible(ports *[]types.PortMapping, allHostPorts, allContainerPorts, currentHostPorts *[65536]bool,
	previousPort *types.PortMapping, port types.PortMapping,
) (*types.PortMapping, error) {
	// no previous port just return the current one
	if previousPort == nil {
		return &port, nil
	}
	if previousPort.ContainerPort+previousPort.Range > port.ContainerPort {
		// calc the new range use the old range and add the difference between the ports
		newRange := port.ContainerPort - previousPort.ContainerPort + port.Range
		// if the newRange is greater than the old range use it
		// this is important otherwise we would could lower the range
		if newRange > previousPort.Range {
			previousPort.Range = newRange
		}
		return previousPort, nil
	}
	// we could not join the ports so we append the old one to the list
	// and return the current port as previous port
	newPort, err := getRandomHostPort(currentHostPorts, *previousPort)
	if err != nil {
		return nil, err
	}
	if err := addPortToUsedPorts(ports, allHostPorts, allContainerPorts, currentHostPorts, &newPort); err != nil {
		return nil, err
	}
	return &port, nil
}

func addPortToUsedPorts(ports *[]types.PortMapping, allHostPorts, allContainerPorts, currentHostPorts *[65536]bool, port *types.PortMapping) error {
	for i := uint16(0); i < port.Range; i++ {
		h := port.HostPort + i
		allHostPorts[h] = true
		if currentHostPorts[h] {
			return fmt.Errorf("host port %q was already assigned in another port mapping", net.JoinHostPort(port.HostIP, strconv.Itoa(int(h)))+"/"+port.Protocol)
		}
		currentHostPorts[h] = true
		c := port.ContainerPort + i
		allContainerPorts[c] = true
	}
	*ports = append(*ports, *port)
	return nil
}

// getRandomHostPort get a random host port mapping for the given port
// the caller has to supply an array with the already used ports
func getRandomHostPort(hostPorts *[65536]bool, port types.PortMapping) (types.PortMapping, error) {
outer:
	for range 15 {
		ranPort, err := utils.GetRandomPort()
		if err != nil {
			return port, err
		}

		// if port range is exceeds max port we cannot use it
		if ranPort+int(port.Range) > 65535 {
			continue
		}

		// check if there is a port in the range which is used
		for j := 0; j < int(port.Range); j++ {
			// port already used
			if hostPorts[ranPort+j] {
				continue outer
			}
		}

		port.HostPort = uint16(ranPort)
		return port, nil
	}

	// add range to error message if needed
	rangePort := ""
	if port.Range > 1 {
		rangePort = fmt.Sprintf("with range %d ", port.Range)
	}

	return port, fmt.Errorf("failed to find an open port to expose container port %d %son the host", port.ContainerPort, rangePort)
}

// Parse port maps to port mappings.
// Returns a set of port mappings, and maps of utilized container and
// host ports.
func ParsePortMapping(portMappings []types.PortMapping, exposePorts map[uint16][]string) ([]types.PortMapping, error) {
	if len(portMappings) == 0 && len(exposePorts) == 0 {
		return nil, nil
	}

	// tempMapping stores the ports without ip and protocol
	type tempMapping struct {
		hostPort      uint16
		containerPort uint16
		rangePort     uint16
	}

	// portMap is a temporary structure to sort all ports
	// the map is hostIp -> protocol -> array of mappings
	portMap := make(map[string]map[string][]tempMapping)

	// allUsedContainerPorts stores all used ports for each protocol
	// the key is the protocol and the array is 65536 elements long for each port.
	allUsedContainerPortsMap := make(map[string]*[65536]bool)
	allUsedHostPortsMap := make(map[string]*[65536]bool)

	// First, we need to validate the ports passed in the specgen
	for _, port := range portMappings {
		// First, check proto
		protocols, err := checkProtocol(port.Protocol)
		if err != nil {
			return nil, err
		}
		if port.HostIP != "" {
			if ip := net.ParseIP(port.HostIP); ip == nil {
				return nil, fmt.Errorf("invalid IP address %q in port mapping", port.HostIP)
			}
		}

		// Validate port numbers and range.
		portRange := port.Range
		if portRange == 0 {
			portRange = 1
		}
		containerPort := port.ContainerPort
		if containerPort == 0 {
			return nil, fmt.Errorf("container port number must be non-0")
		}
		hostPort := port.HostPort
		if uint32(portRange-1)+uint32(containerPort) > 65535 {
			return nil, fmt.Errorf("container port range exceeds maximum allowable port number")
		}
		if uint32(portRange-1)+uint32(hostPort) > 65535 {
			return nil, fmt.Errorf("host port range exceeds maximum allowable port number")
		}

		hostProtoMap, ok := portMap[port.HostIP]
		if !ok {
			hostProtoMap = make(map[string][]tempMapping)
			for _, proto := range []string{protoTCP, protoUDP, protoSCTP} {
				hostProtoMap[proto] = make([]tempMapping, 0)
			}
			portMap[port.HostIP] = hostProtoMap
		}

		p := tempMapping{
			hostPort:      port.HostPort,
			containerPort: port.ContainerPort,
			rangePort:     portRange,
		}

		for _, proto := range protocols {
			hostProtoMap[proto] = append(hostProtoMap[proto], p)
		}
	}

	// we do no longer need the original port mappings
	// set it to 0 length so we can reuse it to populate
	// the slice again while keeping the underlying capacity
	portMappings = portMappings[:0]

	for hostIP, protoMap := range portMap {
		for protocol, ports := range protoMap {
			if len(ports) == 0 {
				continue
			}
			// 1. sort the ports by host port
			// use a small hack to make sure ports with host port 0 are sorted last
			sort.Slice(ports, func(i, j int) bool {
				if ports[i].hostPort == ports[j].hostPort {
					return ports[i].containerPort < ports[j].containerPort
				}
				if ports[i].hostPort == 0 {
					return false
				}
				if ports[j].hostPort == 0 {
					return true
				}
				return ports[i].hostPort < ports[j].hostPort
			})

			allUsedContainerPorts := allUsedContainerPortsMap[protocol]
			if allUsedContainerPorts == nil {
				var temp [65536]bool
				allUsedContainerPorts = &temp
			}
			allUsedHostPorts := allUsedHostPortsMap[protocol]
			if allUsedHostPorts == nil {
				var temp [65536]bool
				allUsedHostPorts = &temp
			}
			var usedHostPorts [65536]bool

			var previousPort *types.PortMapping
			var i int
			for i = 0; i < len(ports); i++ {
				if ports[i].hostPort == 0 {
					// because the ports are sorted and host port 0 is last
					// we can break when we hit 0
					// we will fit them in afterwards
					break
				}
				p := types.PortMapping{
					HostIP:        hostIP,
					Protocol:      protocol,
					HostPort:      ports[i].hostPort,
					ContainerPort: ports[i].containerPort,
					Range:         ports[i].rangePort,
				}
				var err error
				previousPort, err = joinTwoPortsToRangePortIfPossible(&portMappings, allUsedHostPorts,
					allUsedContainerPorts, &usedHostPorts, previousPort, p)
				if err != nil {
					return nil, err
				}
			}
			if previousPort != nil {
				if err := addPortToUsedPorts(&portMappings, allUsedHostPorts,
					allUsedContainerPorts, &usedHostPorts, previousPort); err != nil {
					return nil, err
				}
			}

			// now take care of the hostPort = 0 ports
			previousPort = nil
			for i < len(ports) {
				p := types.PortMapping{
					HostIP:        hostIP,
					Protocol:      protocol,
					ContainerPort: ports[i].containerPort,
					Range:         ports[i].rangePort,
				}
				var err error
				previousPort, err = joinTwoContainerPortsToRangePortIfPossible(&portMappings, allUsedHostPorts,
					allUsedContainerPorts, &usedHostPorts, previousPort, p)
				if err != nil {
					return nil, err
				}
				i++
			}
			if previousPort != nil {
				newPort, err := getRandomHostPort(&usedHostPorts, *previousPort)
				if err != nil {
					return nil, err
				}

				if err := addPortToUsedPorts(&portMappings, allUsedHostPorts,
					allUsedContainerPorts, &usedHostPorts, &newPort); err != nil {
					return nil, err
				}
			}

			allUsedContainerPortsMap[protocol] = allUsedContainerPorts
			allUsedHostPortsMap[protocol] = allUsedHostPorts
		}
	}

	if len(exposePorts) > 0 {
		logrus.Debugf("Adding exposed ports")

		for port, protocols := range exposePorts {
			newProtocols := make([]string, 0, len(protocols))
			for _, protocol := range protocols {
				allUsedContainerPorts := allUsedContainerPortsMap[protocol]
				// Lookup if the container port was already given an explicit mapping,
				// if we have no mappings or the port is not set then we need to add it to a random host port
				if allUsedContainerPorts == nil || !allUsedContainerPorts[port] {
					p := types.PortMapping{
						ContainerPort: port,
						Protocol:      protocol,
						Range:         1,
					}
					allUsedHostPorts := allUsedHostPortsMap[protocol]
					if allUsedHostPorts == nil {
						var temp [65536]bool
						allUsedHostPorts = &temp
					}
					p, err := getRandomHostPort(allUsedHostPorts, p)
					if err != nil {
						return nil, err
					}
					portMappings = append(portMappings, p)
					// Mark this port as used so it doesn't get re-generated
					allUsedHostPorts[p.HostPort] = true
					allUsedHostPortsMap[protocol] = allUsedHostPorts
				} else {
					newProtocols = append(newProtocols, protocol)
				}
			}
			// make sure to delete the key from the map if there are no protocols left
			if len(newProtocols) == 0 {
				delete(exposePorts, port)
			} else {
				exposePorts[port] = newProtocols
			}
		}
	}
	return portMappings, nil
}

func appendProtocolsNoDuplicates(slice []string, protocols []string) []string {
	for _, proto := range protocols {
		if slices.Contains(slice, proto) {
			continue
		}
		slice = append(slice, proto)
	}
	return slice
}

// Make final port mappings for the container
func createPortMappings(s *specgen.SpecGenerator, imageData *libimage.ImageData) ([]types.PortMapping, map[uint16][]string, error) {
	expose := make(map[uint16]string)
	var err error
	if imageData != nil {
		expose, err = GenExposedPorts(imageData.Config.ExposedPorts)
		if err != nil {
			return nil, nil, err
		}
	}

	toExpose := make(map[uint16][]string, len(s.Expose)+len(expose))
	for _, expose := range []map[uint16]string{expose, s.Expose} {
		for port, proto := range expose {
			if port == 0 {
				return nil, nil, fmt.Errorf("cannot expose 0 as it is not a valid port number")
			}
			protocols, err := checkProtocol(proto)
			if err != nil {
				return nil, nil, fmt.Errorf("validating protocols for exposed port %d: %w", port, err)
			}
			toExpose[port] = appendProtocolsNoDuplicates(toExpose[port], protocols)
		}
	}

	publishPorts := toExpose
	if s.PublishExposedPorts == nil || !*s.PublishExposedPorts {
		publishPorts = nil
	}

	finalMappings, err := ParsePortMapping(s.PortMappings, publishPorts)
	if err != nil {
		return nil, nil, err
	}
	return finalMappings, toExpose, nil
}

// Check a string to ensure it is a comma-separated set of valid protocols
func checkProtocol(protocol string) ([]string, error) {
	protocols := make(map[string]struct{})
	// Don't error on duplicates - just deduplicate
	for p := range strings.SplitSeq(protocol, ",") {
		p = strings.ToLower(p)
		switch p {
		case protoTCP, "":
			protocols[protoTCP] = struct{}{}
		case protoUDP:
			protocols[protoUDP] = struct{}{}
		case protoSCTP:
			protocols[protoSCTP] = struct{}{}
		default:
			return nil, fmt.Errorf("unrecognized protocol %q in port mapping", p)
		}
	}

	finalProto := []string{}
	for p := range protocols {
		finalProto = append(finalProto, p)
	}

	// This shouldn't be possible, but check anyways
	if len(finalProto) == 0 {
		return nil, fmt.Errorf("no valid protocols specified for port mapping")
	}

	return finalProto, nil
}

func GenExposedPorts(exposedPorts map[string]struct{}) (map[uint16]string, error) {
	expose := make([]string, 0, len(exposedPorts))
	for e := range exposedPorts {
		expose = append(expose, e)
	}
	toReturn, err := specgenutil.CreateExpose(expose)
	if err != nil {
		return nil, fmt.Errorf("unable to convert image EXPOSE: %w", err)
	}
	return toReturn, nil
}
