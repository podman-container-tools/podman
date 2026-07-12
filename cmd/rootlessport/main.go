//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	rkport "github.com/rootless-containers/rootlesskit/v2/pkg/port"
	rkbuiltin "github.com/rootless-containers/rootlesskit/v2/pkg/port/builtin"
	rkportutil "github.com/rootless-containers/rootlesskit/v2/pkg/port/portutil"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/netns"
	"go.podman.io/common/pkg/rootlessport"
	"golang.org/x/sys/unix"
)

const (
	// ReexecChildKey is used internally for the second reexec
	ReexecChildKey       = "rootlessport-child"
	reexecChildEnvOpaque = "_CONTAINERS_ROOTLESSPORT_CHILD_OPAQUE"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, `too many arguments, rootlessport expects a json config via STDIN`)
		os.Exit(1)
	}
	var err error
	if os.Args[0] == ReexecChildKey {
		err = child()
	} else {
		err = parent()
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func loadConfig(r io.Reader) (*rootlessport.Config, io.ReadCloser, io.WriteCloser, error) {
	stdin, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, nil, err
	}
	var cfg rootlessport.Config
	if err := json.Unmarshal(stdin, &cfg); err != nil {
		return nil, nil, nil, err
	}
	if cfg.NetNSPath == "" {
		return nil, nil, nil, errors.New("missing NetNSPath")
	}
	if cfg.ExitFD <= 0 {
		return nil, nil, nil, errors.New("missing ExitFD")
	}
	exitFile := os.NewFile(uintptr(cfg.ExitFD), "exitfile")
	if exitFile == nil {
		return nil, nil, nil, errors.New("invalid ExitFD")
	}
	if cfg.ReadyFD <= 0 {
		return nil, nil, nil, errors.New("missing ReadyFD")
	}
	readyFile := os.NewFile(uintptr(cfg.ReadyFD), "readyfile")
	if readyFile == nil {
		return nil, nil, nil, errors.New("invalid ReadyFD")
	}
	return &cfg, exitFile, readyFile, nil
}

func parent() error {
	// load config from stdin
	cfg, exitR, readyW, err := loadConfig(os.Stdin)
	if err != nil {
		return err
	}

	socketDir := filepath.Join(cfg.TmpDir, "rp")
	err = os.MkdirAll(socketDir, 0o700)
	if err != nil {
		return err
	}

	// create the parent driver
	stateDir, err := os.MkdirTemp(cfg.TmpDir, "rootlessport")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stateDir)
	driver, err := rkbuiltin.NewParentDriver(&logrusWriter{prefix: "parent: "}, stateDir)
	if err != nil {
		return err
	}
	initComplete := make(chan struct{})
	quit := make(chan struct{})
	errCh := make(chan error)
	// start the parent driver. initComplete will be closed when the child connected to the parent.
	logrus.Infof("Starting parent driver")
	go func() {
		driverErr := driver.RunParentDriver(initComplete, quit, nil)
		if driverErr != nil {
			logrus.WithError(driverErr).Warn("Parent driver exited")
		}
		errCh <- driverErr
		close(errCh)
	}()
	opaque := driver.OpaqueForChild()
	logrus.Infof("opaque=%+v", opaque)
	opaqueJSON, err := json.Marshal(opaque)
	if err != nil {
		return err
	}
	childQuitR, childQuitW, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() {
		// stop the child
		logrus.Info("Stopping child driver")
		if err := childQuitW.Close(); err != nil {
			logrus.WithError(err).Warn("Unable to close childQuitW")
		}
	}()

	// reexec the child process in the child netns
	cmd := exec.Command("/proc/self/exe")
	cmd.Args = []string{ReexecChildKey}
	cmd.Stdin = childQuitR
	cmd.Stdout = &logrusWriter{prefix: "child"}
	cmd.Stderr = cmd.Stdout
	cmd.Env = append(os.Environ(), reexecChildEnvOpaque+"="+string(opaqueJSON))
	childNS, err := netns.GetNS(cfg.NetNSPath)
	if err != nil {
		return err
	}
	if err := childNS.Do(func(_ netns.NetNS) error {
		logrus.Infof("Starting child driver in child netns (%q %v)", cmd.Path, cmd.Args)
		return cmd.Start()
	}); err != nil {
		return err
	}

	childErrCh := make(chan error)
	go func() {
		err := cmd.Wait()
		childErrCh <- err
		close(childErrCh)
	}()

	defer func() {
		if err := unix.Kill(cmd.Process.Pid, unix.SIGTERM); err != nil {
			logrus.WithError(err).Warn("Kill child process")
		}
	}()

	logrus.Info("Waiting for initComplete")
	// wait for the child to connect to the parent
outer:
	for {
		select {
		case <-initComplete:
			logrus.Infof("initComplete is closed; parent and child established the communication channel")
			break outer
		case err := <-childErrCh:
			if err != nil {
				return err
			}
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}

	defer func() {
		logrus.Info("Stopping parent driver")
		quit <- struct{}{}
		if err := <-errCh; err != nil {
			logrus.WithError(err).Warn("Parent driver returned error on exit")
		}
	}()

	// let parent expose ports
	logrus.Infof("Exposing ports %v", cfg.Mappings)
	if err := exposePorts(driver, cfg.Mappings, cfg.ChildIP); err != nil {
		return err
	}

	// we only need to have a socket to reload ports when we are using
	// rootless bridge networking (RootlessCNI is set when netStatus != nil)
	if cfg.RootlessCNI {
		socketfile := filepath.Join(socketDir, cfg.ContainerID)
		// make sure to remove the file if it exists to prevent EADDRINUSE
		_ = os.Remove(socketfile)
		// workaround to bypass the 108 char socket path limit
		// open the fd and use the path to the fd as bind argument
		fd, err := unix.Open(socketDir, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		socket, err := net.ListenUnix("unixpacket", &net.UnixAddr{Name: fmt.Sprintf("/proc/self/fd/%d/%s", fd, cfg.ContainerID), Net: "unixpacket"})
		if err != nil {
			return err
		}
		err = unix.Close(fd)
		// remove the socket file on exit
		defer os.Remove(socketfile)
		if err != nil {
			logrus.Warnf("Failed to close the socketDir fd: %v", err)
		}
		defer socket.Close()
		go serve(socket, driver)
	}

	logrus.Info("Ready")

	// https://github.com/containers/podman/issues/11248
	// Copy /dev/null to stdout and stderr to prevent SIGPIPE errors
	if f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0o755); err == nil {
		unix.Dup2(int(f.Fd()), 1) //nolint:errcheck
		unix.Dup2(int(f.Fd()), 2) //nolint:errcheck
		f.Close()
	}
	// write and close ReadyFD to signal readiness
	if _, err := readyW.Write([]byte("1")); err != nil {
		return err
	}
	if err := readyW.Close(); err != nil {
		return err
	}

	// wait for ExitFD to be closed
	logrus.Info("Waiting for exitfd to be closed")
	if _, err := io.ReadAll(exitR); err != nil {
		return err
	}
	return nil
}

func serve(listener net.Listener, pm rkport.Manager) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// we cannot log this error, stderr is already closed
			continue
		}
		ctx := context.TODO()
		err = handler(ctx, conn, pm)
		if err != nil {
			_, _ = conn.Write([]byte(err.Error()))
		} else {
			_, _ = conn.Write([]byte("OK"))
		}
		conn.Close()
	}
}

func handler(ctx context.Context, conn io.Reader, pm rkport.Manager) error {
	var childIP string
	dec := json.NewDecoder(conn)
	err := dec.Decode(&childIP)
	if err != nil {
		return fmt.Errorf("rootless port failed to decode ports: %w", err)
	}
	portStatus, err := pm.ListPorts(ctx)
	if err != nil {
		return fmt.Errorf("rootless port failed to list ports: %w", err)
	}
	for _, status := range portStatus {
		err = pm.RemovePort(ctx, status.ID)
		if err != nil {
			return fmt.Errorf("rootless port failed to remove port: %w", err)
		}
	}
	// add the ports with the new child IP
	for _, status := range portStatus {
		// set the new child IP
		status.Spec.ChildIP = childIP
		_, err = pm.AddPort(ctx, status.Spec)
		if err != nil {
			return fmt.Errorf("rootless port failed to add port: %w", err)
		}
	}
	return nil
}

func exposePorts(pm rkport.Manager, portMappings []types.PortMapping, childIP string) error {
	ctx := context.TODO()
	for _, port := range portMappings {
		for protocol := range strings.SplitSeq(port.Protocol, ",") {
			hostIP := port.HostIP
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			for i := uint16(0); i < port.Range; i++ {
				spec := rkport.Spec{
					Proto:      protocol,
					ParentIP:   hostIP,
					ParentPort: int(port.HostPort + i),
					ChildPort:  int(port.ContainerPort + i),
					ChildIP:    childIP,
				}

				for _, spec = range splitDualStackSpecIfWsl(spec) {
					if err := validateAndAddPort(ctx, pm, spec); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func validateAndAddPort(ctx context.Context, pm rkport.Manager, spec rkport.Spec) error {
	if err := rkportutil.ValidatePortSpec(spec, nil); err != nil {
		return err
	}
	if _, err := pm.AddPort(ctx, spec); err != nil {
		return err
	}

	return nil
}

func child() error {
	// load the config from the parent
	var opaque map[string]string
	if err := json.Unmarshal([]byte(os.Getenv(reexecChildEnvOpaque)), &opaque); err != nil {
		return err
	}

	// start the child driver
	quit := make(chan struct{})
	errCh := make(chan error)
	go func() {
		d := rkbuiltin.NewChildDriver(os.Stderr)
		dErr := d.RunChildDriver(opaque, quit, "")
		errCh <- dErr
	}()
	defer func() {
		logrus.Info("Stopping child driver")
		quit <- struct{}{}
		if err := <-errCh; err != nil {
			logrus.WithError(err).Warn("Child driver returned error on exit")
		}
	}()

	// wait for stdin to be closed
	if _, err := io.ReadAll(os.Stdin); err != nil {
		return err
	}
	return nil
}

type logrusWriter struct {
	prefix string
}

func (w *logrusWriter) Write(p []byte) (int, error) {
	logrus.Infof("%s%s", w.prefix, string(p))
	return len(p), nil
}
