//go:build linux

// Package slirp4netns contains rootlessport/RootlessKit port mapping helpers.
// The slirp4netns backend itself has been removed; this package is retained
// only for the RLK port-forwarding functions still used by podman.
package slirp4netns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/rootlessport"
	"go.podman.io/common/pkg/servicereaper"
)

// default slirp4netns subnet, used as fallback in GetIP.
const defaultSubnet = "10.0.2.0/24"

type SetupOptions struct {
	// Config used to get helper binary paths and other default options
	Config *config.Config
	// ContainerID is the ID of the container
	ContainerID string
	// Netns path to the netns
	Netns string
	// Ports that should be forwarded
	Ports []types.PortMapping
	// RootlessPortExitPipeR pipe used to exit the rootlessport process.
	// This must be the reading end, the writer must be kept open until you want the
	// process to exit. For podman, conmon will hold the pipe open.
	RootlessPortExitPipeR *os.File
}

type logrusDebugWriter struct {
	prefix string
}

func (w *logrusDebugWriter) Write(p []byte) (int, error) {
	logrus.Debugf("%s%s", w.prefix, string(p))
	return len(p), nil
}

func waitForSync(syncR *os.File, cmd *exec.Cmd, logFile io.ReadSeeker, timeout time.Duration) error {
	prog := filepath.Base(cmd.Path)
	if len(cmd.Args) > 0 {
		prog = cmd.Args[0]
	}
	b := make([]byte, 16)
	for {
		if err := syncR.SetDeadline(time.Now().Add(timeout)); err != nil {
			return fmt.Errorf("setting %s pipe timeout: %w", prog, err)
		}
		// FIXME: return err as soon as proc exits, without waiting for timeout
		_, err := syncR.Read(b)
		if err == nil {
			break
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			// Check if the process is still running.
			var status syscall.WaitStatus
			pid, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
			if err != nil {
				return fmt.Errorf("failed to read %s process status: %w", prog, err)
			}
			if pid != cmd.Process.Pid {
				continue
			}
			if status.Exited() {
				// Seek at the beginning of the file and read all its content
				if _, err := logFile.Seek(0, 0); err != nil {
					logrus.Errorf("Could not seek log file: %q", err)
				}
				logContent, err := io.ReadAll(logFile)
				if err != nil {
					return fmt.Errorf("%s failed: %w", prog, err)
				}
				return fmt.Errorf("%s failed: %q", prog, logContent)
			}
			if status.Signaled() {
				return fmt.Errorf("%s killed by signal", prog)
			}
			continue
		}
		return fmt.Errorf("failed to read from %s sync pipe: %w", prog, err)
	}
	return nil
}

// SetupRootlessPortMappingViaRLK starts the rootlessport process and returns
// its pid.
func SetupRootlessPortMappingViaRLK(opts *SetupOptions, slirpSubnet *net.IPNet, netStatus map[string]types.StatusBlock) (int, error) {
	syncR, syncW, err := os.Pipe()
	if err != nil {
		return 0, fmt.Errorf("failed to open pipe: %w", err)
	}
	defer closeQuiet(syncR)
	defer closeQuiet(syncW)

	logPath := filepath.Join(opts.Config.Engine.TmpDir, fmt.Sprintf("rootlessport-%s.log", opts.ContainerID))
	logFile, err := os.Create(logPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open rootlessport log file %s: %w", logPath, err)
	}
	defer logFile.Close()
	// Unlink immediately the file so we won't need to worry about cleaning it up later.
	// It is still accessible through the open fd logFile.
	if err := os.Remove(logPath); err != nil {
		return 0, fmt.Errorf("delete file %s: %w", logPath, err)
	}

	childIP := GetRootlessPortChildIP(slirpSubnet, netStatus)
	cfg := rootlessport.Config{
		Mappings:    opts.Ports,
		NetNSPath:   opts.Netns,
		ExitFD:      3,
		ReadyFD:     4,
		TmpDir:      opts.Config.Engine.TmpDir,
		ChildIP:     childIP,
		ContainerID: opts.ContainerID,
		RootlessCNI: netStatus != nil,
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return 0, err
	}
	cfgR := bytes.NewReader(cfgJSON)
	var stdout bytes.Buffer
	path, err := opts.Config.FindHelperBinary(rootlessport.BinaryName, false)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(path)
	cmd.Args = []string{rootlessport.BinaryName}

	// Leak one end of the pipe in rootlessport process, the other will be sent to conmon
	cmd.ExtraFiles = append(cmd.ExtraFiles, opts.RootlessPortExitPipeR, syncW)
	cmd.Stdin = cfgR
	// stdout is for human-readable error, stderr is for debug log
	cmd.Stdout = &stdout
	cmd.Stderr = io.MultiWriter(logFile, &logrusDebugWriter{"rootlessport: "})
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start rootlessport process: %w", err)
	}
	defer func() {
		servicereaper.AddPID(cmd.Process.Pid)
		if err := cmd.Process.Release(); err != nil {
			logrus.Errorf("Unable to release rootlessport process: %q", err)
		}
	}()
	if err := waitForSync(syncR, cmd, logFile, 3*time.Second); err != nil {
		stdoutStr := stdout.String()
		if stdoutStr != "" {
			// err contains full debug log and too verbose, so return stdoutStr
			logrus.Debug(err)
			return 0, errors.New("rootlessport " + strings.TrimSuffix(stdoutStr, "\n"))
		}
		return 0, err
	}
	logrus.Debug("rootlessport is ready")
	return cmd.Process.Pid, nil
}

// GetIP returns the slirp ipv4 address based on subnet. If subnet is nil use default subnet.
func GetIP(subnet *net.IPNet) (*net.IP, error) {
	_, slirpSubnet, _ := net.ParseCIDR(defaultSubnet)
	if subnet != nil {
		slirpSubnet = subnet
	}
	expectedIP, err := addToIP(slirpSubnet, uint32(100))
	if err != nil {
		return nil, fmt.Errorf("calculating expected ip: %w", err)
	}
	return expectedIP, nil
}

func addToIP(subnet *net.IPNet, offset uint32) (*net.IP, error) {
	ipFixed := subnet.IP.To4()

	ipInteger := uint32(ipFixed[3]) | uint32(ipFixed[2])<<8 | uint32(ipFixed[1])<<16 | uint32(ipFixed[0])<<24
	ipNewRaw := ipInteger + offset
	if ipNewRaw < ipInteger {
		return nil, fmt.Errorf("integer overflow while calculating ip address offset, %s + %d", ipFixed, offset)
	}
	ipNew := net.IPv4(byte(ipNewRaw>>24), byte(ipNewRaw>>16&0xFF), byte(ipNewRaw>>8)&0xFF, byte(ipNewRaw&0xFF))
	if !subnet.Contains(ipNew) {
		return nil, fmt.Errorf("calculated ip address %s is not within given subnet %s", ipNew.String(), subnet.String())
	}
	return &ipNew, nil
}

func GetRootlessPortChildIP(slirpSubnet *net.IPNet, netStatus map[string]types.StatusBlock) string {
	if slirpSubnet != nil {
		childIP, err := GetIP(slirpSubnet)
		if err != nil {
			return ""
		}
		return childIP.String()
	}

	var ipv6 net.IP
	for _, status := range netStatus {
		for _, netInt := range status.Interfaces {
			for _, netAddress := range netInt.Subnets {
				ipv4 := netAddress.IPNet.IP.To4()
				if ipv4 != nil {
					return ipv4.String()
				}
				ipv6 = netAddress.IPNet.IP
			}
		}
	}
	if ipv6 != nil {
		return ipv6.String()
	}
	return ""
}

func closeQuiet(f *os.File) {
	if err := f.Close(); err != nil {
		logrus.Errorf("Unable to close file %s: %q", f.Name(), err)
	}
}
