package lock

import (
	"fmt"
	"path/filepath"

	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/storage/pkg/lockfile"
)

func GetMachineLock(name string, machineConfigDir string) (*lockfile.LockFile, error) {
	lockPath := filepath.Join(machineConfigDir, name+".lock")
	lock, err := lockfile.GetLockFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("creating lockfile for VM: %w", err)
	}
	return lock, nil
}

const machineStartLockName = "machine-start.lock"

// GetMachineStartLock is a lock only used to prevent starting different machines at the same time,
// This is required as most provides support at max 1 running VM and to check this race free we
// cannot allows starting two machine.
func GetMachineStartLock() (*lockfile.LockFile, error) {
	lockDir, err := env.GetGlobalDataDir()
	if err != nil {
		return nil, err
	}

	lock, err := lockfile.GetLockFile(filepath.Join(lockDir, machineStartLockName))
	if err != nil {
		return nil, err
	}
	return lock, nil
}
