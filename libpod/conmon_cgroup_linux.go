//go:build !remote

package libpod

import (
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

// mustCreateConmonCgroup says whether podman, rather than systemd or the user,
// is in charge of putting conmon into its own cgroup.
func (c *Container) mustCreateConmonCgroup() bool {
	mustCreateCgroup := !c.config.NoCgroups

	// If cgroup creation is disabled - just signal.
	switch c.config.CgroupsMode {
	case "disabled", "no-conmon", cgroupSplit:
		mustCreateCgroup = false
	}

	// $INVOCATION_ID is set by systemd when running as a service.
	if c.runtime.RemoteURI() == "" && os.Getenv("INVOCATION_ID") != "" {
		mustCreateCgroup = false
	}

	return mustCreateCgroup
}

// conmonCgroupLogLevel returns the level a failure to set up the conmon cgroup
// should be reported at.
func (c *Container) conmonCgroupLogLevel() logrus.Level {
	// Usually rootless users are not allowed to configure cgroupfs.
	// There are cases though, where it is allowed, e.g. if the cgroup
	// is manually configured and chowned).  Avoid detecting all
	// such cases and simply use a lower log level.
	if rootless.IsRootless() {
		return logrus.InfoLevel
	}
	return logrus.WarnLevel
}

// conmonScopeUnit returns the slice and the transient unit name of the conmon
// scope used by the systemd cgroup manager.
func (c *Container) conmonScopeUnit() (slice, unitName string) {
	slice = c.CgroupParent()
	splitParent := strings.Split(slice, "/")
	if strings.HasSuffix(slice, ".slice") && len(splitParent) > 1 {
		slice = splitParent[len(splitParent)-1]
	}
	return slice, createUnitName("libpod-conmon", c.ID())
}

// conmonCgroupfsPath returns the conmon cgroup path used by the cgroupfs cgroup
// manager.
func (c *Container) conmonCgroupfsPath() string {
	return filepath.Join(c.config.CgroupParent, "conmon")
}

// moveToConmonCgroup creates the container's conmon cgroup and moves pid into
// it.  It is a no-op when podman does not manage that cgroup at all.
//
// Reporting the error is up to the caller, and every caller should only log it:
// ending up in the wrong cgroup is not a reason to fail the container, and
// rootless users routinely cannot configure cgroupfs at all.  Use
// conmonCgroupLogLevel() to pick a level for that.
func (c *Container) moveToConmonCgroup(pid int) error {
	if !c.mustCreateConmonCgroup() {
		return nil
	}

	// TODO: This should be a switch - we are not guaranteed that
	// there are only 2 valid cgroup managers
	if c.CgroupManager() == config.SystemdCgroupsManager {
		slice, unitName := c.conmonScopeUnit()
		logrus.Infof("Running conmon under slice %s and unitName %s", slice, unitName)
		return systemd.RunUnderSystemdScope([]int{pid}, slice, unitName)
	}

	cgroupResources, err := GetLimits(c.LinuxResources())
	if err != nil {
		return fmt.Errorf("could not get ctr resources: %w", err)
	}
	control, err := cgroups.New(c.conmonCgroupfsPath(), &cgroupResources)
	if err != nil {
		return err
	}
	// we need to remove this defer and delete the cgroup once conmon exits
	// maybe need a conmon monitor?
	return control.AddPid(pid)
}
