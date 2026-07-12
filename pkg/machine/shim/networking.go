package shim

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/connection"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/ports"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

const (
	dockerSock           = "/var/run/docker.sock"
	defaultGuestSock     = "/run/user/%d/podman/podman.sock"
	dockerConnectTimeout = 5 * time.Second
)

var (
	ErrNotRunning      = errors.New("machine not in running state")
	ErrSSHNotListening = errors.New("machine is not listening on ssh port")
)

func startHostForwarder(mc *vmconfigs.MachineConfig, provider vmconfigs.VMProvider, dirs *define.MachineDirs, hostSocks []string) error {
	forwardUser := mc.SSH.RemoteUsername

	// TODO should this go up the stack higher or
	// the guestSock is "inside" the guest machine
	guestSock := fmt.Sprintf(defaultGuestSock, mc.HostUser.UID)
	if mc.HostUser.Rootful {
		guestSock = "/run/podman/podman.sock"
		forwardUser = "root"
	}

	cfg, err := config.Default()
	if err != nil {
		return err
	}

	binary, err := cfg.FindHelperBinary(machine.ForwarderBinaryName, false)
	if err != nil {
		return err
	}

	cmd := gvproxy.NewGvproxyCommand()

	// GvProxy PID file path is now derived
	runDir := dirs.RuntimeDir
	cmd.PidFile = filepath.Join(runDir.GetPath(), "gvproxy.pid")

	cmd.LogFile = filepath.Join(runDir.GetPath(), "gvproxy.log")

	cmd.SSHPort = mc.SSH.Port

	// Windows providers listen on multiple sockets since they do not involve links
	for _, hostSock := range hostSocks {
		cmd.AddForwardSock(hostSock)
		cmd.AddForwardDest(guestSock)
		cmd.AddForwardUser(forwardUser)
		cmd.AddForwardIdentity(mc.SSH.IdentityPath)
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Debug = true
		logrus.Debug(cmd)
	}

	// This allows a provider to perform additional setup as well as
	// add in any provider specific options for gvproxy
	if err := provider.StartNetworking(mc, &cmd); err != nil {
		return err
	}

	c := cmd.Cmd(binary)

	logrus.Debugf("gvproxy command-line: %s %s", binary, strings.Join(cmd.ToCmdline(), " "))
	if err := c.Start(); err != nil {
		return fmt.Errorf("unable to execute: %q: %w", cmd.ToCmdline(), err)
	}

	return nil
}

func startNetworking(mc *vmconfigs.MachineConfig, provider vmconfigs.VMProvider) (string, machine.APIForwardingState, error) {
	// Check if SSH port is in use, and reassign if necessary
	if !ports.IsLocalPortAvailable(mc.SSH.Port) {
		logrus.Warnf("detected port conflict on machine ssh port [%d], reassigning", mc.SSH.Port)
		if err := reassignSSHPort(mc, provider); err != nil {
			return "", 0, err
		}
	}

	// Provider has its own networking code path (e.g. WSL)
	if provider.UseProviderNetworkSetup() {
		return "", 0, provider.StartNetworking(mc, nil)
	}

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return "", 0, err
	}

	hostSocks, forwardSock, forwardingState, err := setupMachineSockets(mc, dirs)
	if err != nil {
		return "", 0, err
	}

	if err := startHostForwarder(mc, provider, dirs, hostSocks); err != nil {
		return "", 0, err
	}

	return forwardSock, forwardingState, nil
}

// conductVMReadinessCheck checks to make sure the machine is in the proper state
// and that SSH is up and running
func conductVMReadinessCheck(mc *vmconfigs.MachineConfig, maxBackoffs int, backoff time.Duration, stateF func() (define.Status, error)) (connected bool, sshError error, err error) {
	for i := range maxBackoffs {
		if i > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}
		state, err := stateF()
		if err != nil {
			return false, nil, err
		}
		if state != define.Running {
			sshError = ErrNotRunning
			continue
		}
		if !isListening(mc.SSH.Port) {
			sshError = ErrSSHNotListening
			continue
		}

		// Also make sure that SSH is up and running.  The
		// ready service's dependencies don't fully make sure
		// that clients can SSH into the machine immediately
		// after boot.
		//
		// CoreOS users have reported the same observation but
		// the underlying source of the issue remains unknown.

		if sshError = machine.LocalhostSSHSilent(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, []string{"true"}); sshError != nil {
			logrus.Debugf("SSH readiness check for machine failed: %v", sshError)
			continue
		}
		connected = true
		sshError = nil
		break
	}
	return connected, sshError, err
}

func reassignSSHPort(mc *vmconfigs.MachineConfig, provider vmconfigs.VMProvider) error {
	newPort, err := ports.AllocateMachinePort()
	if err != nil {
		return err
	}

	success := false
	defer func() {
		if !success {
			if err := ports.ReleaseMachinePort(newPort); err != nil {
				logrus.Warnf("could not release port allocation as part of failure rollback (%d): %s", newPort, err.Error())
			}
		}
	}()

	// Write a transient invalid port, to force a retry on failure
	oldPort := mc.SSH.Port
	mc.SSH.Port = 0
	if err := mc.Write(); err != nil {
		return err
	}

	if err := ports.ReleaseMachinePort(oldPort); err != nil {
		logrus.Warnf("could not release current ssh port allocation (%d): %s", oldPort, err.Error())
	}

	// Update the backend's settings if relevant (e.g. WSL)
	if err := provider.UpdateSSHPort(mc, newPort); err != nil {
		return err
	}

	mc.SSH.Port = newPort
	if err := connection.UpdateConnectionPairPort(mc.Name, newPort, mc.HostUser.UID, mc.SSH.RemoteUsername, mc.SSH.IdentityPath); err != nil {
		return fmt.Errorf("could not update remote connection configuration: %w", err)
	}

	// Write updated port back
	if err := mc.Write(); err != nil {
		return err
	}

	// inform defer routine not to release the port
	success = true

	return nil
}

func isListening(port int) bool {
	// Check if we can dial it
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", "127.0.0.1", port), 10*time.Millisecond)
	if err != nil {
		return false
	}
	if err := conn.Close(); err != nil {
		logrus.Error(err)
	}
	return true
}
