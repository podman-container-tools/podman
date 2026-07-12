//go:build dragonfly || freebsd || linux || netbsd || openbsd || darwin

package shim

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func setupMachineSockets(mc *vmconfigs.MachineConfig, dirs *define.MachineDirs) ([]string, string, machine.APIForwardingState, error) {
	hostSocket, err := mc.APISocket()
	if err != nil {
		return nil, "", 0, err
	}

	forwardSock, state, err := setupForwardingLinks(hostSocket, dirs.DataDir)
	if err != nil {
		return nil, "", 0, err
	}
	return []string{hostSocket.GetPath()}, forwardSock, state, nil
}

func setupForwardingLinks(hostSocket, dataDir *define.VMFile) (string, machine.APIForwardingState, error) {
	// Sets up a cooperative link structure to help a separate privileged
	// service manage /var/run/docker.sock (currently only on MacOS via
	// podman-mac-helper, but potentially other OSs in the future).
	//
	// The linking pattern is:
	//
	// /var/run/docker.sock (link) -> user global sock (link) -> machine sock
	//
	// This allows the helper to only have to maintain one constant target to
	// the user, which can be repositioned without updating docker.sock.
	//
	// Since these link locations are global/shared across multiple machine
	// instances, they must coordinate on the winner. The scheme is first come
	// first serve, whoever is actively answering on the socket first wins. All
	// other machine instances backs off. As soon as the winner is no longer
	// active another machine instance start will become the new active winner.
	// The same applies to a competing container runtime trying to use
	// /var/run/docker.sock, if the socket is in use by another runtime, podman
	// machine will back off. In the start message "Losing" machine instances
	// will instead advertise the direct machine socket, while "winning"
	// instances will simply note they listen on the standard
	// /var/run/docker.sock address. The APIForwardingState return value is
	// returned by this function to indicate how the start message should behave

	// Skip any OS not supported for helper usage
	if !dockerClaimSupported() {
		return hostSocket.GetPath(), machine.ClaimUnsupported, nil
	}

	// Verify the helper system service was installed and report back if not
	if !dockerClaimHelperInstalled() {
		return hostSocket.GetPath(), machine.NotInstalled, nil
	}

	dataPath := filepath.Dir(dataDir.GetPath())
	userGlobalSocket, err := define.NewMachineFile(filepath.Join(dataPath, "podman.sock"), nil)
	if err != nil {
		return "", 0, err
	}

	// Setup the user global socket if not in use
	// (e.g ~/.local/share/containers/podman/machine/podman.sock)
	if !alreadyLinked(hostSocket.GetPath(), userGlobalSocket.GetPath()) {
		if checkSockInUse(userGlobalSocket.GetPath()) {
			return hostSocket.GetPath(), machine.MachineLocal, nil
		}

		_ = userGlobalSocket.Delete()

		if err := os.Symlink(hostSocket.GetPath(), userGlobalSocket.GetPath()); err != nil {
			logrus.Warnf("could not create user global API forwarding link: %s", err.Error())
			return hostSocket.GetPath(), machine.MachineLocal, nil
		}
	}

	// Setup /var/run/docker.sock if not in use
	if !alreadyLinked(userGlobalSocket.GetPath(), dockerSock) {
		if checkSockInUse(dockerSock) {
			return hostSocket.GetPath(), machine.MachineLocal, nil
		}

		if !claimDockerSock() {
			logrus.Warn("podman helper is installed, but was not able to claim the global docker sock")
			return hostSocket.GetPath(), machine.MachineLocal, nil
		}
	}

	return dockerSock, machine.DockerGlobal, nil
}

func alreadyLinked(target string, link string) bool {
	read, err := os.Readlink(link)
	return err == nil && read == target
}

func checkSockInUse(sock string) bool {
	if info, err := os.Stat(sock); err == nil && info.Mode()&fs.ModeSocket == fs.ModeSocket {
		_, err = net.DialTimeout("unix", dockerSock, dockerConnectTimeout)
		return err == nil
	}

	return false
}
