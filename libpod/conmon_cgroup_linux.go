//go:build !remote

package libpod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/systemd"
	"go.podman.io/podman/v6/pkg/rootless"
)

// Create systemd unit name for cgroup scopes.
func createUnitName(prefix string, name string) string {
	return fmt.Sprintf("%s-%s.scope", prefix, name)
}

// mustCreateConmonCgroup returns true when podman, rather than systemd or the
// user, is responsible for creating the cgroup that conmon runs in.
func (c *Container) mustCreateConmonCgroup() bool {
	if c.config.NoCgroups {
		return false
	}

	switch c.config.CgroupsMode {
	case "disabled", "no-conmon", cgroupSplit:
		return false
	}

	// $INVOCATION_ID is set by systemd when running as a service.
	if c.runtime.RemoteURI() == "" && os.Getenv("INVOCATION_ID") != "" {
		return false
	}

	return true
}

// conmonCgroupLogLevel returns the level at which failures to set up the conmon
// cgroup are reported.  Usually rootless users are not allowed to configure
// cgroupfs.  There are cases though, where it is allowed, e.g. if the cgroup
// is manually configured and chowned).  Avoid detecting all such cases and
// simply use a lower log level.
func (c *Container) conmonCgroupLogLevel() logrus.Level {
	if rootless.IsRootless() {
		return logrus.InfoLevel
	}
	return logrus.WarnLevel
}

// conmonScopeUnit returns the systemd slice and scope unit name of the
// container's conmon cgroup.  Only meaningful with the systemd cgroup manager.
func (c *Container) conmonScopeUnit() (slice, unitName string) {
	slice = c.CgroupParent()
	splitParent := strings.Split(slice, "/")
	if strings.HasSuffix(slice, ".slice") && len(splitParent) > 1 {
		slice = splitParent[len(splitParent)-1]
	}
	return slice, createUnitName("libpod-conmon", c.ID())
}

// conmonCgroupfsPath returns the path of the container's conmon cgroup.  Only
// meaningful with the cgroupfs cgroup manager.
func (c *Container) conmonCgroupfsPath() string {
	return filepath.Join(c.config.CgroupParent, "conmon")
}

// netHelperPids returns the pids of the per-container network helpers that
// belong in the conmon cgroup.  Helpers without a pid are skipped: the state of
// a container created by an older podman does not record them, and not
// accounting one of them is preferable to failing the container over it.
//
// The shared rootless netns pasta is deliberately not included.  It outlives
// any single container and is moved to user.slice on purpose.
func (c *Container) netHelperPids() []int {
	var pids []int
	if c.pastaResult != nil && c.pastaResult.Pid > 0 {
		pids = append(pids, c.pastaResult.Pid)
	}
	if c.rootlessPortPid > 0 {
		pids = append(pids, c.rootlessPortPid)
	}
	return pids
}

// moveToConmonCgroup moves pids into the container's conmon cgroup, creating it
// first when create is set.  It reports whether the cgroup now exists, which is
// false both when creating it failed and when podman does not manage it at all.
//
// Reporting the error is up to the caller, and every caller should only log it:
// ending up in the wrong cgroup is not a reason to fail the container, and
// rootless users routinely cannot configure cgroupfs at all.  Use
// conmonCgroupLogLevel() to pick a level for that.
//
// The first pid is the important one, the rest are network helpers.  A helper
// that died in the meantime must never keep the first one out of the cgroup.
func (c *Container) moveToConmonCgroup(create bool, pids ...int) (bool, error) {
	if len(pids) == 0 || !c.mustCreateConmonCgroup() {
		return false, nil
	}

	// TODO: This should be a switch - we are not guaranteed that
	// there are only 2 valid cgroup managers
	if c.CgroupManager() == config.SystemdCgroupsManager {
		slice, unitName := c.conmonScopeUnit()
		if !create {
			return false, systemd.AddPidsToSystemdScope(unitName, pids...)
		}
		logrus.Infof("Running conmon under slice %s and unitName %s", slice, unitName)
		err := systemd.RunUnderSystemdScope(pids, slice, unitName)
		if err != nil && len(pids) > 1 {
			// One of the helpers is likely gone already, retry without
			// them rather than leave conmon outside of its own scope.
			logrus.Infof("Failed to add conmon and the network helpers to the systemd sandbox cgroup, retrying with conmon only: %v", err)
			err = systemd.RunUnderSystemdScope(pids[:1], slice, unitName)
		}
		return err == nil, err
	}

	var control *cgroups.CgroupControl
	var err error
	if create {
		cgroupResources, rerr := GetLimits(c.LinuxResources())
		if rerr != nil {
			return false, fmt.Errorf("could not get ctr resources: %w", rerr)
		}
		control, err = cgroups.New(c.conmonCgroupfsPath(), &cgroupResources)
	} else {
		control, err = cgroups.Load(c.conmonCgroupfsPath())
	}
	if err != nil {
		return false, err
	}

	// we need to remove this defer and delete the cgroup once conmon exits
	// maybe need a conmon monitor?
	var errs []error
	for _, pid := range pids {
		if err := control.AddPid(pid); err != nil {
			errs = append(errs, fmt.Errorf("add process %d: %w", pid, err))
		}
	}
	// The cgroup itself is there even if some of the processes could not be
	// moved into it.
	return create, errors.Join(errs...)
}
