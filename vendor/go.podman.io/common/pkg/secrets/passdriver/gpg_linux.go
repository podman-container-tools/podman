//go:build linux

package passdriver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"go.podman.io/storage/pkg/unshare"
)

// gpgCommand runs GPG with the invoking user's numeric identity when Podman has
// mapped that user to UID/GID 0 in its rootless user namespace.  GnuPG derives
// its runtime socket directory from getuid(), so preserving the numeric identity
// lets it find the invoking user's existing agent and keyboxd sockets.
func gpgCommand(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
	return gpgCommandForIdentity(ctx, os.Geteuid(), unshare.GetRootlessUID(), unshare.GetRootlessGID(), program, args...)
}

func gpgCommandForIdentity(ctx context.Context, euid, uid, gid int, program string, args ...string) (*exec.Cmd, error) {
	nested, err := rootlessUserNamespace(euid, uid, gid)
	if err != nil {
		return nil, err
	}
	if !nested {
		return exec.CommandContext(ctx, program, args...), nil
	}

	cmd := exec.CommandContext(ctx, program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: uid, HostID: 0, Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: gid, HostID: 0, Size: 1}},
	}
	return cmd, nil
}

func rootlessUserNamespace(euid, uid, gid int) (bool, error) {
	// Only namespace UID 0 maps to the invoking host user's UID. If GPG ran
	// as another parent-namespace UID, it would map to a subordinate host UID
	// and could not access the user's sockets under /run/user/<uid>/gnupg.
	if euid != 0 || uid == 0 {
		return false, nil
	}
	if uid < 0 || gid < 0 {
		return false, fmt.Errorf("invalid rootless UID/GID %d:%d", uid, gid)
	}
	return true, nil
}
