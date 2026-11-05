//go:build !remote

package abi

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/systemd"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/unshare"
	"golang.org/x/sys/unix"
)

// Default path for system runtime state
const defaultRunPath = "/run"

func (ic *ContainerEngine) SetupRootless(_ context.Context, noMoveProcess bool, cgroupMode string) error {
	runsUnderSystemd := systemd.RunsOnSystemd()
	if !runsUnderSystemd {
		isPid1 := os.Getpid() == 1
		if _, found := os.LookupEnv("container"); isPid1 || found {
			if err := cgroups.MaybeMoveToSubCgroup(); err != nil {
				// it is a best effort operation, so just print the
				// error for debugging purposes.
				logrus.Debugf("Could not move to subcgroup: %v", err)
			}
		}
	}

	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return err
	}

	// check for both euid == 0 and CAP_SYS_ADMIN because we may be running in a container with CAP_SYS_ADMIN set.
	if os.Geteuid() == 0 && hasCapSysAdmin {
		// do it only after podman has already re-execed and running with uid==0.
		configureCgroup := cgroupMode != "disabled"
		if configureCgroup {
			ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
			if err != nil {
				logrus.Infof("Failed to detect the owner for the current cgroup: %v", err)
			}
			if !ownsCgroup {
				conf, err := ic.Config(context.Background())
				if err != nil {
					return err
				}
				unitName := fmt.Sprintf("podman-%d.scope", os.Getpid())
				if runsUnderSystemd || conf.Engine.CgroupManager == config.SystemdCgroupsManager {
					if err := systemd.RunUnderSystemdScope(os.Getpid(), "containers.slice", unitName); err != nil {
						logrus.Debugf("Failed to add podman to systemd sandbox cgroup: %v", err)
					}
				}
			}
		}

		// return early as we are already re-exec or root here so no need to join the rootless userns.
		return nil
	}

	stateDir, err := util.GetRootlessStateDir()
	if err != nil {
		return fmt.Errorf("could not get rootless state directory: %w", err)
	}

	became, ret, err := rootless.TryJoinPauseProcess(stateDir)
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}

	// if there is no pid file, try to join existing containers, and create a pause process.
	ctrs, err := ic.Libpod.GetRunningContainers()
	if err != nil {
		return err
	}

	paths := make([]string, 0, len(ctrs))
	for _, ctr := range ctrs {
		paths = append(paths, ctr.ConfigNoCopy().ConmonPidFile)
	}

	if len(paths) > 0 {
		became, ret, err = rootless.TryJoinFromFilePaths(stateDir, paths)
		// TryJoinFromFilePaths fails with ESRCH when the PID are all not valid anymore
		// In this case create a new userns.
		if errors.Is(err, unix.ESRCH) {
			logrus.Warnf("Failed to join existing conmon namespace, creating a new rootless podman user namespace. If there are existing container running please stop them with %q to reset the namespace", os.Args[0]+" system migrate")
			became, ret, err = rootless.BecomeRootInUserNS(stateDir)
		}
	} else {
		logrus.Info("Creating a new rootless user namespace")
		became, ret, err = rootless.BecomeRootInUserNS(stateDir)
	}

	if err != nil {
		return fmt.Errorf("fatal error, invalid internal status, unable to create a new pause process: %w. Try running %q and if that doesn't work reboot to recover", err, os.Args[0]+" system migrate")
	}
	if !noMoveProcess {
		systemd.MovePauseProcessToScope(rootless.GetPausePidPath(stateDir))
	}
	if became {
		os.Exit(ret)
	}

	logrus.Error("Internal error, failed to re-exec podman into user namespace without error. This should never happen, if you see this please report a bug")
	return nil
}
