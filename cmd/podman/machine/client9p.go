//go:build linux && (amd64 || arm64)

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mdlayher/vsock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var client9pCommand = &cobra.Command{
	Args:              cobra.ExactArgs(2),
	Use:               "client9p PORT DIR",
	Hidden:            true,
	Short:             "Mount a remote directory using 9p over hvsock",
	Long:              "Connect to the given hvsock port using 9p and mount the served filesystem at the given directory",
	RunE:              remoteDirClient,
	ValidArgsFunction: completion.AutocompleteNone,
	Example:           `podman system client9p 55000 /mnt`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: client9pCommand,
		Parent:  machineCmd,
	})
}

func remoteDirClient(_ *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("error parsing port number: %w", err)
	}

	if err := client9p(uint32(port), args[1]); err != nil {
		return err
	}

	return nil
}

// This is Linux-only as we only intend for this function to be used inside the
// `podman machine` VM, which is guaranteed to be Linux.
func client9p(portNum uint32, mountPath string) error {
	cleanPath, err := filepath.Abs(mountPath)
	if err != nil {
		return fmt.Errorf("absolute path for %s: %w", mountPath, err)
	}
	mountPath = cleanPath

	// Mountpath needs to exist and be a directory
	stat, err := os.Stat(mountPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", mountPath, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("path %s is not a directory", mountPath)
	}

	logrus.Infof("Going to mount 9p on vsock port %d to directory %s", portNum, mountPath)

	// The server is starting at the same time.
	// Perform up to 5 retries with a backoff.
	var (
		conn    *vsock.Conn
		retries = 20
	)
	for range retries {
		// Host connects to non-hypervisor processes on the host running the VM.
		conn, err = vsock.Dial(vsock.Host, portNum, nil)
		// If errors.Is worked on this error, we could detect non-timeout errors.
		// But it doesn't. So retry 5 times regardless.
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("dialing vsock port %d: %w", portNum, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logrus.Errorf("Error closing vsock: %v", err)
		}
	}()

	// vsock doesn't give us direct access to the underlying FD. That's kind
	// of inconvenient, because we have to pass it off to mount.
	// However, it does give us the ability to get a syscall.RawConn, which
	// has a method that allows us to run a function that takes the FD
	// number as an argument.
	// Which ought to be good enough? Probably?
	// Overall, this is gross and I hate it, but I don't see a better way.
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("getting vsock raw conn: %w", err)
	}
	errChan := make(chan error, 1)
	runMount := func(fd uintptr) {
		vsock := os.NewFile(fd, "vsock")
		if vsock == nil {
			errChan <- fmt.Errorf("could not convert vsock fd to os.File")
			return
		}

		// This is ugly, but it lets us use real kernel mount code,
		// instead of maintaining our own FUSE 9p implementation.
		cmd := exec.Command("mount", "-t", "9p", "-o", "trans=fd,rfdno=3,wfdno=3,version=9p2000.L", "9p", mountPath)
		cmd.ExtraFiles = []*os.File{vsock}

		output, err := cmd.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("running mount: %w\nOutput: %s", err, string(output))
		} else {
			logrus.Debugf("Mount output: %s", string(output))
			logrus.Infof("Mounted directory %s using 9p", mountPath)
		}

		errChan <- err
		close(errChan)
	}
	if err := rawConn.Control(runMount); err != nil {
		return fmt.Errorf("running mount function for dir %s: %w", mountPath, err)
	}

	if err := <-errChan; err != nil {
		return fmt.Errorf("mounting filesystem %s: %w", mountPath, err)
	}

	logrus.Infof("Mount of filesystem %s successful", mountPath)

	return nil
}
