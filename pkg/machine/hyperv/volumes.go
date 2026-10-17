//go:build windows

package hyperv

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/machine/windows"
)

func startShares(mc *vmconfigs.MachineConfig) error {
	for _, mount := range mc.Mounts {
		args, err := startShareMountArgs(mount, logrus.IsLevelEnabled(logrus.DebugLevel))
		if err != nil {
			return err
		}

		if err := machine.LocalhostSSH(mc.SSH.RemoteUsername, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, args); err != nil {
			return err
		}
	}
	return nil
}

func startShareMountArgs(mount *vmconfigs.Mount, debug bool) ([]string, error) {
	cleanTarget := path.Clean(mount.Target)
	var args []string
	requiresChattr := !strings.HasPrefix(cleanTarget, "/home") && !strings.HasPrefix(cleanTarget, "/mnt")
	if requiresChattr {
		args = append(args, "sudo", "chattr", "-i", "/", "; ")
	}
	args = append(args, "sudo", "mkdir", "-p", strconv.Quote(cleanTarget), "; ")
	if requiresChattr {
		args = append(args, "sudo", "chattr", "+i", "/", "; ")
	}

	args = append(args, "sudo")
	if mount.ReadOnly {
		// Pass read-only preference via environment variable rather than a CLI flag.
		// Older guest podman versions ignore the env var, while newer versions
		// read it to mount the filesystem read-only.
		args = append(args, "CONTAINERS_MACHINE_9P_READ_ONLY=1")
	}
	args = append(args, "podman")
	if debug {
		args = append(args, "--log-level=debug")
	}
	clientArgs, err := client9pArgs(mount)
	if err != nil {
		return nil, err
	}
	args = append(args, clientArgs...)
	return args, nil
}

// client9pArgs returns the podman command arguments to mount a 9p share over vsock.
func client9pArgs(mount *vmconfigs.Mount) ([]string, error) {
	if mount.VSockNumber == nil {
		return nil, errors.New("cannot start 9p shares with undefined vsock number")
	}
	return []string{"machine", "client9p", fmt.Sprintf("%d", *mount.VSockNumber), strconv.Quote(mount.Target)}, nil
}

func createShares(mc *vmconfigs.MachineConfig) (err error) {
	fileServerVsocks, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(vsock.Fileserver)
	if err != nil {
		return fmt.Errorf("failed to load existing file server vsock registry entries: %w", err)
	}
	for i, mount := range mc.Mounts {
		var testVsock *vsock.HVSockRegistryEntry

		// Check if there's an existing file server vsock entry that can be reused for the current mount.
		if i < len(fileServerVsocks) {
			testVsock = fileServerVsocks[i]
		} else {
			// If no existing vsock entry can be reused, a new one must be created.
			// Creating a new HVSockRegistryEntry requires administrator privileges.
			if !windows.HasAdminRights() {
				if i == 0 {
					return ErrHypervRegistryInitRequiresElevation
				}
				return ErrHypervRegistryUpdateRequiresElevation
			}
			testVsock, err = vsock.NewHVSockRegistryEntry(vsock.Fileserver, false)
			if err != nil {
				return err
			}
		}

		mount.VSockNumber = &testVsock.Port
		mc.HyperVHypervisor.FileserverVSocks = append(mc.HyperVHypervisor.FileserverVSocks, *testVsock)
		logrus.Debugf("Going to share directory %s via 9p on vsock %d", mount.Source, testVsock.Port)
	}
	return nil
}
