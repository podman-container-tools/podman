package systemd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/storage/pkg/unshare"
	"golang.org/x/sys/unix"
)

var (
	runsOnSystemdOnce sync.Once
	runsOnSystemd     bool
)

// RunsOnSystemd returns whether the system is using systemd.
func RunsOnSystemd() bool {
	runsOnSystemdOnce.Do(func() {
		// per sd_booted(3), check for this dir
		fd, err := os.Stat("/run/systemd/system")
		runsOnSystemd = err == nil && fd.IsDir()
	})
	return runsOnSystemd
}

func moveProcessPIDFileToScope(pidPath, slice, scope string) error {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		// do not raise an error if the file doesn't exist
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read pid file: %w", err)
	}
	pid, err := strconv.ParseUint(string(data), 10, 0)
	if err != nil {
		return fmt.Errorf("cannot parse pid file %s: %w", pidPath, err)
	}

	return moveProcessToScope(int(pid), slice, scope)
}

func moveProcessToScope(pid int, slice, scope string) error {
	err := RunUnderSystemdScope([]int{pid}, slice, scope)
	// If the PID is not valid anymore, do not return an error.
	if isPidGoneErr(err) {
		return nil
	}
	return err
}

// isPidGoneErr returns true when err reports that one of the pids we asked
// systemd to act on does not exist (anymore).  Such a process obviously cannot
// be moved anywhere, which is not something callers need to care about.
func isPidGoneErr(err error) bool {
	if err == nil {
		return false
	}
	if dbusErr, ok := err.(dbus.Error); ok {
		if dbusErr.Name == "org.freedesktop.DBus.Error.UnixProcessIdUnknown" {
			return true
		}
	}
	return errors.Is(err, unix.ESRCH)
}

// systemdConn opens a connection to the systemd manager responsible for this
// process: the user manager when running rootless, the system one otherwise.
// The caller is responsible for closing the returned connection.
func systemdConn() (*systemdDbus.Conn, error) {
	if uid := unshare.GetRootlessUID(); uid != 0 {
		return cgroups.UserConnection(uid)
	}
	return systemdDbus.NewWithContext(context.Background())
}

// MoveRootlessNetnsProcessToUserSlice moves the pasta process for the rootless netns
// into a different scope so that systemd does not kill it with a container.
func MoveRootlessNetnsProcessToUserSlice(pid int) error {
	randBytes := make([]byte, 4)
	_, err := rand.Read(randBytes)
	if err != nil {
		return err
	}
	return moveProcessToScope(pid, "user.slice", fmt.Sprintf("rootless-netns-%x.scope", randBytes))
}

// MovePauseProcessToScope moves the pause process used for rootless mode to keep the namespaces alive to
// a separate scope.
func MovePauseProcessToScope(pausePidPath string) {
	var err error

	for range 10 {
		randBytes := make([]byte, 4)
		_, err = rand.Read(randBytes)
		if err != nil {
			logrus.Errorf("failed to read random bytes: %v", err)
			continue
		}
		err = moveProcessPIDFileToScope(pausePidPath, "user.slice", fmt.Sprintf("podman-pause-%x.scope", randBytes))
		if err == nil {
			return
		}
	}

	if err != nil {
		_, err2 := cgroups.IsCgroup2UnifiedMode()
		if err2 != nil {
			logrus.Warnf("Failed to detect if running with cgroup unified: %v", err)
		}
		if RunsOnSystemd() {
			logrus.Warnf("Failed to add pause process to systemd sandbox cgroup: %v", err)
		}
	}
}

// RunUnderSystemdScope adds the specified pids to a systemd scope.  They must
// all already be running, as the scope is created with all of them at once.
func RunUnderSystemdScope(pids []int, slice string, unitName string) error {
	conn, err := systemdConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	u := make([]uint32, 0, len(pids))
	for _, p := range pids {
		u = append(u, uint32(p))
	}

	properties := []systemdDbus.Property{
		systemdDbus.PropSlice(slice),
		newProp("PIDs", u),
		newProp("Delegate", true),
		newProp("DefaultDependencies", false),
	}
	ch := make(chan string)
	_, err = conn.StartTransientUnitContext(context.Background(), unitName, "replace", properties, ch)
	if err != nil {
		// The unit already exists, so attach the processes to it instead.
		if aerr := attachPidsToUnit(conn, unitName, u); aerr == nil {
			return nil
		}
		// On errors return the original error message we got from StartTransientUnit.
		return err
	}

	// Block until job is started
	<-ch

	return nil
}

// AddPidsToSystemdScope attaches already running processes to an existing
// systemd scope.  The scope must have been created with Delegate=true, which is
// the case for every scope RunUnderSystemdScope creates.  Unlike
// RunUnderSystemdScope this never creates the unit; use it when the scope is
// known to exist already.
func AddPidsToSystemdScope(unitName string, pids ...int) error {
	if len(pids) == 0 {
		return nil
	}
	conn, err := systemdConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	u := make([]uint32, 0, len(pids))
	for _, p := range pids {
		u = append(u, uint32(p))
	}
	return attachPidsToUnit(conn, unitName, u)
}

// attachPidsToUnit asks systemd to migrate pids into an existing delegated
// unit.  Going through systemd rather than writing cgroup.procs ourselves keeps
// systemd's bookkeeping in sync and works rootless, where a direct write is
// usually refused by cgroup v2 delegation containment: the common ancestor of
// our own cgroup and the target is typically a root owned user-$UID.slice.
func attachPidsToUnit(conn *systemdDbus.Conn, unitName string, pids []uint32) error {
	err := conn.AttachProcessesToUnit(context.Background(), unitName, "/", pids)
	if err == nil {
		return nil
	}
	if isPidGoneErr(err) {
		if len(pids) == 1 {
			return nil
		}
		// systemd gave up on the pid that is gone and we cannot tell which one
		// that was, so the ones after it were never attached.  Retry them one
		// by one; a single pid that is gone is not an error.
		for _, pid := range pids {
			if aerr := attachPidsToUnit(conn, unitName, []uint32{pid}); aerr != nil {
				return aerr
			}
		}
		return nil
	}

	// AttachProcessesToUnit only exists since systemd v237 and is refused for
	// units that are not delegated.  Fall back to moving the processes by hand.
	props, perr := conn.GetUnitTypePropertiesContext(context.Background(), unitName, "Scope")
	if perr != nil {
		return err
	}
	cgroup, ok := props["ControlGroup"].(string)
	if !ok || cgroup == "" {
		return err
	}
	if merr := cgroups.MoveUnderCgroup(cgroup, "", pids); merr != nil {
		// Return the error from the preferred code path.
		return err
	}
	return nil
}

func newProp(name string, units any) systemdDbus.Property {
	return systemdDbus.Property{
		Name:  name,
		Value: dbus.MakeVariant(units),
	}
}
