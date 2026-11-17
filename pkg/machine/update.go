//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func UpdatePodmanDockerSockService(mc *vmconfigs.MachineConfig) error {
	content := ignition.GetPodmanDockerTmpConfig(mc.HostUser.UID, mc.HostUser.Rootful, false)
	command := fmt.Sprintf("'echo %q > %s'", content, ignition.PodmanDockerTmpConfPath)
	args := []string{"sudo", "bash", "-c", command}
	if err := LocalhostSSH(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, args); err != nil {
		logrus.Warnf("Could not update internal docker sock config")
		return err
	}

	args = []string{"sudo", "systemd-tmpfiles", "--create", "--prefix=/run/docker.sock"}
	if err := LocalhostSSH(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, args); err != nil {
		logrus.Warnf("Could not create internal docker sock")
		return err
	}

	return nil
}
