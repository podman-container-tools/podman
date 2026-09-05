//go:build linux && cgo

package rootless

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	gosignal "os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/moby/sys/capability"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/storage/pkg/idtools"
	pmount "go.podman.io/storage/pkg/mount"
	"go.podman.io/storage/pkg/unshare"
	"golang.org/x/sys/unix"
)

/*
#cgo remote CFLAGS: -Wall -Werror -DDISABLE_JOIN_SHORTCUT
#include <stdlib.h>
#include <sys/types.h>
extern uid_t rootless_uid();
extern uid_t rootless_gid();
extern int reexec_in_user_namespace(int ready, char *state_dir);
extern int reexec_in_user_namespace_wait(int pid, int options);
extern int reexec_userns_join(int pid, char *state_dir);
extern int is_fd_inherited(int fd);
*/
import "C"

const (
	numSig = 65 // max number of signals
)

func init() {
	rootlessUIDInit := int(C.rootless_uid())
	rootlessGIDInit := int(C.rootless_gid())
	if rootlessUIDInit != 0 {
		// we need this if we joined the user+mount namespace from the C code.
		if err := os.Setenv("_CONTAINERS_USERNS_CONFIGURED", "done"); err != nil {
			logrus.Errorf("Failed to set environment variable %s as %s", "_CONTAINERS_USERNS_CONFIGURED", "done")
		}
		if err := os.Setenv("_CONTAINERS_ROOTLESS_UID", strconv.Itoa(rootlessUIDInit)); err != nil {
			logrus.Errorf("Failed to set environment variable %s as %d", "_CONTAINERS_ROOTLESS_UID", rootlessUIDInit)
		}
		if err := os.Setenv("_CONTAINERS_ROOTLESS_GID", strconv.Itoa(rootlessGIDInit)); err != nil {
			logrus.Errorf("Failed to set environment variable %s as %d", "_CONTAINERS_ROOTLESS_GID", rootlessGIDInit)
		}
	}
}

func runInUser() error {
	return os.Setenv("_CONTAINERS_USERNS_CONFIGURED", "done")
}

var (
	isRootlessOnce sync.Once
	isRootless     bool
)

// IsRootless tells us if we are running in rootless mode
func IsRootless() bool {
	// unshare.IsRootless() is used to check if a user namespace is required.
	// Here we need to make sure that nested podman instances act
	// as if they have root privileges and pick paths on the host
	// that would normally be used for root.
	return unshare.IsRootless() && unshare.GetRootlessUID() > 0
}

// GetRootlessUID returns the UID of the user in the parent userNS
func GetRootlessUID() int {
	return unshare.GetRootlessUID()
}

// GetRootlessGID returns the GID of the user in the parent userNS
func GetRootlessGID() int {
	return unshare.GetRootlessGID()
}

func tryMappingTool(uid bool, pid int, hostID int, mappings []idtools.IDMap) error {
	tool := "newuidmap"
	mode := os.ModeSetuid
	cap := capability.CAP_SETUID
	idtype := "setuid"
	if !uid {
		tool = "newgidmap"
		mode = os.ModeSetgid
		cap = capability.CAP_SETGID
		idtype = "setgid"
	}
	path, err := exec.LookPath(tool)
	if err != nil {
		return fmt.Errorf("command required for rootless mode with multiple IDs: %w", err)
	}

	appendTriplet := func(l []string, a, b, c int) []string {
		return append(l, strconv.Itoa(a), strconv.Itoa(b), strconv.Itoa(c))
	}

	args := []string{path, strconv.Itoa(pid)}
	args = appendTriplet(args, 0, hostID, 1)
	for _, i := range mappings {
		if hostID >= i.HostID && hostID < i.HostID+i.Size {
			what := "UID"
			where := "/etc/subuid"
			if !uid {
				what = "GID"
				where = "/etc/subgid"
			}
			return fmt.Errorf("invalid configuration: the specified mapping %d:%d in %q includes the user %s", i.HostID, i.Size, where, what)
		}
		args = appendTriplet(args, i.ContainerID+1, i.HostID, i.Size)
	}
	cmd := exec.Cmd{
		Path: path,
		Args: args,
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		logrus.Errorf("running `%s`: %s", strings.Join(args, " "), output)
		errorStr := fmt.Sprintf("cannot set up namespace using %q", path)
		if isSet, err := unshare.IsSetID(cmd.Path, mode, cap); err != nil {
			logrus.Errorf("Failed to check for %s on %s: %v", idtype, path, err)
		} else if !isSet {
			errorStr = fmt.Sprintf("%s: should have %s or have filecaps %s", errorStr, idtype, idtype)
		}
		return fmt.Errorf("%v: %w", errorStr, err)
	}
	return nil
}

// joinUserAndMountNS re-exec podman in a new userNS and join the user and mount
// namespace of the specified PID without looking up its parent.  Useful to join directly
// the conmon process.
func joinUserAndMountNS(pid uint, stateDir string) (bool, int, error) {
	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return false, 0, err
	}
	if (os.Geteuid() == 0 && hasCapSysAdmin) || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
		return false, 0, nil
	}

	cStateDir := C.CString(stateDir)
	defer C.free(unsafe.Pointer(cStateDir))

	pidC := C.reexec_userns_join(C.int(pid), cStateDir)
	if int(pidC) < 0 {
		return false, -1, fmt.Errorf("cannot re-exec process to join the existing user namespace")
	}

	return waitAndProxySignalsToChild(pidC)
}

// GetConfiguredMappings returns the additional IDs configured for the current user.
func GetConfiguredMappings(quiet bool) ([]idtools.IDMap, []idtools.IDMap, error) {
	var uids, gids []idtools.IDMap
	username := os.Getenv("USER")
	if username == "" {
		var id string
		if os.Geteuid() == 0 {
			id = strconv.Itoa(GetRootlessUID())
		} else {
			id = strconv.Itoa(os.Geteuid())
		}
		userID, err := user.LookupId(id)
		if err == nil {
			username = userID.Username
		}
	}
	mappings, err := idtools.NewIDMappings(username, username)
	if err != nil {
		logLevel := logrus.ErrorLevel
		if quiet || (os.Geteuid() == 0 && GetRootlessUID() == 0) {
			logLevel = logrus.DebugLevel
		}
		logrus.StandardLogger().Logf(logLevel, "cannot find UID/GID for user %s: %v - check rootless mode in man pages.", username, err)
	} else {
		uids = mappings.UIDs()
		gids = mappings.GIDs()
	}
	return uids, gids, nil
}

func copyMappings(from, to string) error {
	// when running as non-root always go through the newuidmap/newgidmap
	// configuration since this is the expectation when running on Kubernetes
	if os.Geteuid() != 0 {
		return errors.New("copying mappings is allowed only for root")
	}
	content, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	// Both runc and crun check whether the current process is in a user namespace
	// by looking up 4294967295 in /proc/self/uid_map.  If the mappings would be
	// copied as they are, the check in the OCI runtimes would fail.  So just split
	// it in two different ranges.
	if bytes.Contains(content, []byte("4294967295")) {
		content = []byte("0 0 1\n1 1 4294967294\n")
	}
	return os.WriteFile(to, content, 0o600)
}

// saveDeviceGIDsFromArgs reads --device flags from os.Args and saves
// each device's real host GID to a file in stateDir before entering
// the user namespace. After re-exec, device GIDs may appear as 65534
// (overflow) if unmapped, so we need to save them while still on host.
func SaveDeviceGIDsFromArgs(stateDir string) {
	deviceGIDs := make(map[string]int)

	args := os.Args
	for i, arg := range args {
		var devicePath string
		if arg == "--device" && i+1 < len(args) {
			devicePath = args[i+1]
		} else if len(arg) > 9 && arg[:9] == "--device=" {
			devicePath = arg[9:]
		}
		if devicePath == "" {
			continue
		}
		// Strip options like /dev/foo:bar:rw
		if idx := strings.Index(devicePath, ":"); idx >= 0 {
			devicePath = devicePath[:idx]
		}
		var st unix.Stat_t
		if err := unix.Stat(devicePath, &st); err != nil {
			logrus.Debugf("saveDeviceGIDsFromArgs: cannot stat %s: %v", devicePath, err)
			continue
		}
		// If it's a directory, walk it and save GIDs for each device file
		if st.Mode&unix.S_IFMT == unix.S_IFDIR {
			_ = filepath.WalkDir(devicePath, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.Type()&os.ModeDevice == os.ModeDevice {
					var dst unix.Stat_t
					if err := unix.Stat(p, &dst); err == nil {
						deviceGIDs[p] = int(dst.Gid)
						logrus.Debugf("saveDeviceGIDsFromArgs: %s has host GID %d", p, dst.Gid)
					}
				}
				return nil
			})
		} else {
			deviceGIDs[devicePath] = int(st.Gid)
			logrus.Debugf("saveDeviceGIDsFromArgs: %s has host GID %d", devicePath, st.Gid)
		}
	}

	if len(deviceGIDs) == 0 {
		return
	}

	var lines []string
	for path, gid := range deviceGIDs {
		lines = append(lines, fmt.Sprintf("%s:%d", path, gid))
	}
	deviceGIDsFile := stateDir + "/device_gids"
	if mkdirErr := os.MkdirAll(stateDir, 0o700); mkdirErr == nil {
		if writeErr := os.WriteFile(deviceGIDsFile, []byte(strings.Join(lines, "\n")), 0o600); writeErr != nil {
			logrus.Debugf("saveDeviceGIDsFromArgs: failed to write %s: %v", deviceGIDsFile, writeErr)
		} else {
			logrus.Debugf("saveDeviceGIDsFromArgs: saved to %s", deviceGIDsFile)
		}
	}
}

func becomeRootInUserNS(stateDir string) (_ bool, _ int, retErr error) {
	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return false, 0, err
	}

	if (os.Geteuid() == 0 && hasCapSysAdmin) || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
		if os.Getenv("_CONTAINERS_USERNS_CONFIGURED") == "init" {
			return false, 0, runInUser()
		}
		return false, 0, nil
	}

	if _, inContainer := os.LookupEnv("container"); !inContainer {
		if mounts, err := pmount.GetMounts(); err == nil {
			for _, m := range mounts {
				if m.Mountpoint == "/" {
					isShared := false
					for o := range strings.SplitSeq(m.Optional, ",") {
						if strings.HasPrefix(o, "shared:") {
							isShared = true
							break
						}
					}
					if !isShared {
						logrus.Warningf("%q is not a shared mount, this could cause issues or missing mounts with rootless containers", m.Mountpoint)
					}
					break
				}
			}
		}
	}

	cStateDir := C.CString(stateDir)
	defer C.free(unsafe.Pointer(cStateDir))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		return false, -1, err
	}
	r, w := os.NewFile(uintptr(fds[0]), "sync host"), os.NewFile(uintptr(fds[1]), "sync child")

	var pid int

	defer errorhandling.CloseQuiet(r)
	defer errorhandling.CloseQuiet(w)
	defer func() {
		toWrite := []byte("0")
		if retErr != nil {
			toWrite = []byte("1")
		}
		if _, err := w.Write(toWrite); err != nil {
			logrus.Errorf("Failed to write byte 0: %q", err)
		}
		if retErr != nil && pid > 0 {
			if err := unix.Kill(pid, unix.SIGKILL); err != nil {
				if err != unix.ESRCH {
					logrus.Errorf("Failed to clean up process %d: %v", pid, err)
				}
			}
			C.reexec_in_user_namespace_wait(C.int(pid), 0)
		}
	}()

	// Save host groups to file BEFORE re-exec.
	// After reexec_in_user_namespace, euid becomes 0 and os.Getgroups()
	// returns namespace-mapped groups, not real host groups.
	// addDevice reads this file to check if user can access a device's group.
	if hostGroups, err := os.Getgroups(); err == nil {
		parts := make([]string, len(hostGroups))
		for i, g := range hostGroups {
			parts[i] = strconv.Itoa(g)
		}
		groupsFile := stateDir + "/host_groups"
		if mkdirErr := os.MkdirAll(stateDir, 0o755); mkdirErr == nil {
			if writeErr := os.WriteFile(groupsFile, []byte(strings.Join(parts, ",")), 0o600); writeErr != nil {
				logrus.Debugf("Failed to save host groups to file: %v", writeErr)
			} else {
				logrus.Debugf("Saved host groups to %s: %s", groupsFile, strings.Join(parts, ","))
			}
		}
	}

	// Save device GIDs from --device flags BEFORE re-exec.
	// After re-exec device GIDs may be unmapped (appear as 65534).
	SaveDeviceGIDsFromArgs(stateDir)

	pidC := C.reexec_in_user_namespace(C.int(r.Fd()), cStateDir)
	pid = int(pidC)
	if pid < 0 {
		return false, -1, fmt.Errorf("cannot re-exec process")
	}

	uids, gids, err := GetConfiguredMappings(false)
	if err != nil {
		return false, -1, err
	}

	uidMap := fmt.Sprintf("/proc/%d/uid_map", pid)
	gidMap := fmt.Sprintf("/proc/%d/gid_map", pid)

	uidsMapped := false

	if err := copyMappings("/proc/self/uid_map", uidMap); err == nil {
		uidsMapped = true
	}

	if uids != nil && !uidsMapped {
		err := tryMappingTool(true, pid, os.Geteuid(), uids)
		// If some mappings were specified, do not ignore the error
		if err != nil && len(uids) > 0 {
			return false, -1, err
		}
		uidsMapped = err == nil
	}
	if !uidsMapped {
		logrus.Warnf("Using rootless single mapping into the namespace. This might break some images. Check /etc/subuid and /etc/subgid for adding sub*ids if not using a network user")
		setgroups := fmt.Sprintf("/proc/%d/setgroups", pid)
		err = os.WriteFile(setgroups, []byte("deny\n"), 0o666)
		if err != nil {
			return false, -1, fmt.Errorf("cannot write setgroups file: %w", err)
		}
		logrus.Debugf("write setgroups file exited with 0")

		err = os.WriteFile(uidMap, fmt.Appendf(nil, "%d %d 1\n", 0, os.Geteuid()), 0o666)
		if err != nil {
			return false, -1, fmt.Errorf("cannot write uid_map: %w", err)
		}
		logrus.Debugf("write uid_map exited with 0")
	}

	gidsMapped := false
	if err := copyMappings("/proc/self/gid_map", gidMap); err == nil {
		gidsMapped = true
	}
	if gids != nil && !gidsMapped {
		err := tryMappingTool(false, pid, os.Getegid(), gids)
		// If some mappings were specified, do not ignore the error
		if err != nil && len(gids) > 0 {
			return false, -1, err
		}
		gidsMapped = err == nil
	}
	if !gidsMapped {
		err = os.WriteFile(gidMap, fmt.Appendf(nil, "%d %d 1\n", 0, os.Getegid()), 0o666)
		if err != nil {
			return false, -1, fmt.Errorf("cannot write gid_map: %w", err)
		}
	}

	_, err = w.WriteString("0")
	if err != nil {
		return false, -1, fmt.Errorf("write to sync pipe: %w", err)
	}

	b := make([]byte, 1)
	_, err = w.Read(b)
	if err != nil {
		return false, -1, fmt.Errorf("read from sync pipe: %w", err)
	}

	if b[0] == '2' {
		// We have lost the race for writing the PID file, as probably another
		// process created a namespace and wrote the PID.
		// Try to join it.
		pausePidPath := stateDir + "/pause.pid"
		data, err := os.ReadFile(pausePidPath)
		if err == nil {
			var pid uint64
			pid, err = strconv.ParseUint(string(data), 10, 0)
			if err == nil {
				return joinUserAndMountNS(uint(pid), "")
			}
		}
		return false, -1, fmt.Errorf("setting up the process: %w", err)
	}

	if b[0] != '0' {
		return false, -1, errors.New("setting up the process")
	}

	return waitAndProxySignalsToChild(pidC)
}

func waitAndProxySignalsToChild(pid C.int) (bool, int, error) {
	signals := []os.Signal{}
	for sig := range numSig {
		if sig == int(unix.SIGTSTP) {
			continue
		}
		signals = append(signals, unix.Signal(sig))
	}

	// Disable all existing signal handlers, from now forward everything to the child and let
	// it deal with it. All we do is to wait and propagate the exit code from the child to our parent.
	gosignal.Reset()
	c := make(chan os.Signal, len(signals))
	gosignal.Notify(c, signals...)
	go func() {
		for s := range c {
			if s == unix.SIGCHLD || s == unix.SIGPIPE {
				continue
			}

			if err := unix.Kill(int(pid), s.(unix.Signal)); err != nil {
				if err != unix.ESRCH {
					logrus.Errorf("Failed to propagate signal to child process %d: %v", int(pid), err)
				}
			}
		}
	}()

	ret := C.reexec_in_user_namespace_wait(pid, 0)
	// child exited reset our signal proxy handler
	gosignal.Reset()
	if ret < 0 {
		return false, -1, errors.New("waiting for the re-exec process")
	}

	return true, int(ret), nil
}

// BecomeRootInUserNS re-exec podman in a new userNS.  It returns whether podman was re-executed
// into a new user namespace and the return code from the re-executed podman process.
// If podman was re-executed the caller needs to propagate the error code returned by the child
// process.
func BecomeRootInUserNS(stateDir string) (bool, int, error) {
	return becomeRootInUserNS(stateDir)
}

// isPauseProcess checks if the given PID has _PODMAN_PAUSE=1 in its environment.
// It is a best-effort check; any errors are silently ignored and it returns false.
func isPauseProcess(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return false
	}
	for entry := range bytes.SplitSeq(data, []byte{0}) {
		if string(entry) == "_PODMAN_PAUSE=1" {
			return true
		}
	}
	return false
}

// TryJoinFromFilePaths attempts to join the namespaces of the pid files in paths.
// This is useful when there are already running containers and we
// don't have a pause process yet.  We can use the paths to the conmon
// processes to attempt joining their namespaces.
func TryJoinFromFilePaths(stateDir string, paths []string) (bool, int, error) {
	var lastErr error

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}

		pid, err := strconv.Atoi(string(data))
		if err != nil {
			lastErr = fmt.Errorf("cannot parse file %q: %w", path, err)
			continue
		}

		if pid > 0 && unix.Kill(pid, 0) == nil {
			joined, ret, err := joinUserAndMountNS(uint(pid), stateDir)
			if err == nil {
				return joined, ret, nil
			}
			if !isPauseProcess(pid) {
				logrus.Warningf("pause.pid file refers to PID %d which is not a pause process, the process may have exited and the PID been recycled. Removing %s", pid, path)
				os.Remove(path)
				lastErr = err
				continue
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return false, 0, lastErr
	}
	return false, 0, fmt.Errorf("could not find any running process: %w", unix.ESRCH)
}

// IsFdInherited checks whether the fd is opened and valid to use
func IsFdInherited(fd int) bool {
	return int(C.is_fd_inherited(C.int(fd))) > 0
}
