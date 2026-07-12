//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/libpod/define"
	"golang.org/x/sys/unix"
)

// Timeout before declaring that runtime has failed to kill a given
// container
const killContainerTimeout = 5 * time.Second

// ociError is used to parse the OCI runtime JSON log.  It is not part of the
// OCI runtime specifications, it follows what runc does
type ociError struct {
	Level string `json:"level,omitempty"`
	Time  string `json:"time,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

// Bind ports to keep them closed on the host
func bindPorts(ports []types.PortMapping) ([]*os.File, error) {
	var files []*os.File
	sctpWarning := true
	for _, port := range ports {
		isV6 := false
		if port.HostIP != "" {
			isV6 = net.ParseIP(port.HostIP).To4() == nil
		}
		protocols := strings.SplitSeq(port.Protocol, ",")
		for protocol := range protocols {
			for i := uint16(0); i < port.Range; i++ {
				f, err := bindPort(protocol, port.HostIP, port.HostPort+i, isV6, &sctpWarning)
				if err != nil {
					// close all open ports in case of early error so we do not
					// rely on the garbage collector to close them
					for _, f := range files {
						f.Close()
					}
					return nil, err
				}
				if f != nil {
					files = append(files, f)
				}
			}
		}
	}
	return files, nil
}

// bindPort reserves a port on the host using socket+bind without listen.
// Dual-stack bind by default unless hostIP is specified.
func bindPort(protocol, hostIP string, port uint16, isV6 bool, sctpWarning *bool) (*os.File, error) {
	switch protocol {
	case "tcp", "udp":
		sockType := unix.SOCK_STREAM
		if protocol == "udp" {
			sockType = unix.SOCK_DGRAM
		}

		domain, sa, err := buildSockAddr(hostIP, port, isV6)
		if err != nil {
			return nil, err
		}

		fd, err := unix.Socket(domain, sockType|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			// If hostIP == "" and IPv6 is not supported, fall back to IPv4
			if hostIP == "" && errors.Is(err, unix.EAFNOSUPPORT) {
				return bindPortV4Fallback(protocol, sockType, port)
			}
			return nil, fmt.Errorf("cannot create socket for %s port %d: %w", protocol, port, err)
		}

		if err := setupSocketOpts(fd, domain, hostIP); err != nil {
			unix.Close(fd)
			return nil, err
		}

		if err := unix.Bind(fd, sa); err != nil {
			unix.Close(fd)
			return nil, fmt.Errorf("cannot bind %s port %s: %w", protocol, net.JoinHostPort(hostIP, strconv.FormatUint(uint64(port), 10)), err)
		}

		return os.NewFile(uintptr(fd), fmt.Sprintf("reservation-%s-%d", protocol, port)), nil

	case "sctp":
		if *sctpWarning {
			logrus.Info("Port reservation for SCTP is not supported")
			*sctpWarning = false
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown protocol %s", protocol)
	}
}

func buildSockAddr(hostIP string, port uint16, isV6 bool) (int, unix.Sockaddr, error) {
	// default behaviour when hostIP == "" is to bind dual-stack
	// if hostIP != ""; determine the stack using the address specified
	if hostIP == "" {
		return unix.AF_INET6, &unix.SockaddrInet6{Port: int(port)}, nil
	}
	ip := net.ParseIP(hostIP)
	if ip == nil {
		return 0, nil, fmt.Errorf("invalid IP address: %s", hostIP)
	}
	if isV6 {
		sa := &unix.SockaddrInet6{Port: int(port)}
		copy(sa.Addr[:], ip.To16())
		return unix.AF_INET6, sa, nil
	}
	sa := &unix.SockaddrInet4{Port: int(port)}
	copy(sa.Addr[:], ip.To4())
	return unix.AF_INET, sa, nil
}

func setupSocketOpts(fd, domain int, hostIP string) error {
	if domain == unix.AF_INET6 {
		v6only := 1
		if hostIP == "" {
			v6only = 0
		}
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, v6only); err != nil {
			return fmt.Errorf("cannot set IPV6_V6ONLY: %w", err)
		}
	}
	return nil
}

func bindPortV4Fallback(protocol string, sockType int, port uint16) (*os.File, error) {
	fd, err := unix.Socket(unix.AF_INET, sockType|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot create socket for %s port %d: %w", protocol, port, err)
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{Port: int(port)}); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("cannot bind %s port %s: %w", protocol, net.JoinHostPort("", strconv.FormatUint(uint64(port), 10)), err)
	}
	return os.NewFile(uintptr(fd), fmt.Sprintf("reservation-%s-%d", protocol, port)), nil
}

func getOCIRuntimeError(name, runtimeMsg string) error {
	includeFullOutput := logrus.GetLevel() == logrus.DebugLevel

	if match := regexp.MustCompile("(?i).*permission denied.*|.*operation not permitted.*").FindString(runtimeMsg); match != "" {
		errStr := match
		if includeFullOutput {
			errStr = runtimeMsg
		}
		return fmt.Errorf("%s: %s: %w", name, strings.Trim(errStr, "\n"), define.ErrOCIRuntimePermissionDenied)
	}
	if match := regexp.MustCompile("(?i).*executable file not found in.*|.*no such file or directory.*|.*open executable.*").FindString(runtimeMsg); match != "" {
		errStr := match
		if includeFullOutput {
			errStr = runtimeMsg
		}
		return fmt.Errorf("%s: %s: %w", name, strings.Trim(errStr, "\n"), define.ErrOCIRuntimeNotFound)
	}
	if match := regexp.MustCompile("`/proc/[a-z0-9-].+/attr.*`").FindString(runtimeMsg); match != "" {
		errStr := match
		if includeFullOutput {
			errStr = runtimeMsg
		}
		if strings.HasSuffix(match, "/exec`") {
			return fmt.Errorf("%s: %s: %w", name, strings.Trim(errStr, "\n"), define.ErrSetSecurityAttribute)
		} else if strings.HasSuffix(match, "/current`") {
			return fmt.Errorf("%s: %s: %w", name, strings.Trim(errStr, "\n"), define.ErrGetSecurityAttribute)
		}
		return fmt.Errorf("%s: %s: %w", name, strings.Trim(errStr, "\n"), define.ErrSecurityAttribute)
	}
	return fmt.Errorf("%s: %s: %w", name, strings.Trim(runtimeMsg, "\n"), define.ErrOCIRuntime)
}
