//go:build !remote

package libpod

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/util"
)

func (r *Runtime) stopPauseProcess() error {
	if rootless.IsRootless() {
		stateDir, err := util.GetRootlessStateDir()
		if err != nil {
			return fmt.Errorf("could not get rootless state directory: %w", err)
		}

		nsHandlesPath := rootless.GetNamespaceHandlesPath(stateDir)
		if err := os.Remove(nsHandlesPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("Failed to remove namespace handles file %s: %v", nsHandlesPath, err)
		}

		pausePidPath := rootless.GetPausePidPath(stateDir)
		data, err := os.ReadFile(pausePidPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("cannot read pause process pid file: %w", err)
		}
		pausePid, err := strconv.Atoi(string(data))
		if err != nil {
			return fmt.Errorf("cannot parse pause pid file %s: %w", pausePidPath, err)
		}
		if err := os.Remove(pausePidPath); err != nil {
			return fmt.Errorf("cannot delete pause pid file %s: %w", pausePidPath, err)
		}
		if err := syscall.Kill(pausePid, syscall.SIGKILL); err != nil {
			return err
		}
	}
	return nil
}
