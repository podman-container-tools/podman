//go:build linux || freebsd || windows

package qemu

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/storage/pkg/fileutils"
)

// VirtiofsdSpawner spawns an instance of virtiofsd
type virtiofsdSpawner struct {
	runtimeDir *define.VMFile
	binaryPath string
	mountCount uint
}

func newVirtiofsdSpawner(runtimeDir *define.VMFile) (*virtiofsdSpawner, error) {
	var binaryPath string
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	binaryPath, err = cfg.FindHelperBinary("virtiofsd", true)
	if err != nil {
		return nil, fmt.Errorf("failed to find virtiofsd: %w", err)
	}
	return &virtiofsdSpawner{binaryPath: binaryPath, runtimeDir: runtimeDir}, nil
}

// createVirtiofsCmd returns a new command instance configured to launch virtiofsd.
func (v *virtiofsdSpawner) createVirtiofsCmd(directory, socketPath string) *exec.Cmd {
	cmd := exec.Command(v.binaryPath,
		"--sandbox", "none",
		"--socket-path", socketPath,
		"--shared-dir", ".",
		// We don't need seccomp filtering; we trust our workloads. This incidentally
		// works around issues like https://gitlab.com/virtio-fs/virtiofsd/-/merge_requests/200.
		"--seccomp=none",
	)
	// This sets things up so that the `.` we passed in the arguments is the target directory
	cmd.Dir = directory
	// Quiet the daemon by default
	cmd.Env = append(cmd.Environ(), "RUST_LOG=ERROR")
	cmd.Stdin = nil
	cmd.Stdout = nil
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Stderr = os.Stderr
	}
	return cmd
}

type virtiofsdHelperCmd struct {
	command exec.Cmd
	socket  *define.VMFile
}

// spawnForMount returns on success a combination of qemu commandline and child process for virtiofsd
func (v *virtiofsdSpawner) spawnForMount(hostmnt *vmconfigs.Mount) ([]string, *virtiofsdHelperCmd, error) {
	logrus.Debugf("Initializing virtiofsd mount for %s", hostmnt.Source)
	// By far the most common failure to spawn virtiofsd will be a typo'd source directory,
	// so let's synchronously check that ourselves here.
	if err := fileutils.Exists(hostmnt.Source); err != nil {
		return nil, nil, fmt.Errorf("failed to access virtiofs source directory %s", hostmnt.Source)
	}
	virtiofsChar := fmt.Sprintf("virtiofschar%d", v.mountCount)
	virtiofsCharPath, err := v.runtimeDir.AppendToNewVMFile(virtiofsChar, nil)
	if err != nil {
		return nil, nil, err
	}

	qemuCommand := []string{
		"-chardev", fmt.Sprintf("socket,id=%s,path=%s", virtiofsChar, virtiofsCharPath.Path),
		"-device", fmt.Sprintf("vhost-user-fs-pci,queue-size=1024,chardev=%s,tag=%s", virtiofsChar, hostmnt.Tag),
	}
	// TODO: Honor hostmnt.readonly somehow here (add an option to virtiofsd)
	virtiofsdCmd := v.createVirtiofsCmd(hostmnt.Source, virtiofsCharPath.Path)
	if err := virtiofsdCmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start virtiofsd")
	}
	// Wait for the socket
	for {
		if err := fileutils.Exists(virtiofsCharPath.Path); err == nil {
			break
		}
		logrus.Debugf("waiting for virtiofsd socket %q", virtiofsCharPath.Path)
		time.Sleep(time.Millisecond * 100)
	}
	// Increment our count of mounts which are used to create unique names for the devices
	v.mountCount += 1
	return qemuCommand, &virtiofsdHelperCmd{
		command: *virtiofsdCmd,
		socket:  virtiofsCharPath,
	}, nil
}
