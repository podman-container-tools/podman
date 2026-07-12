//go:build linux

package cgroups

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs2"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/unshare"
	"golang.org/x/sys/unix"
)

var (
	// ErrCgroupDeleted means the cgroup was deleted.
	ErrCgroupDeleted = errors.New("cgroup deleted")
	ErrStatCgroup    = errors.New("no cgroup available for gathering user statistics")

	isUnifiedOnce sync.Once
	isUnified     bool
	isUnifiedErr  error
)

// CgroupControl controls a cgroup hierarchy.
type CgroupControl struct {
	config  *cgroups.Cgroup
	systemd bool
}

// statFunc is a function that gathers statistics for a cgroup controller.
type statFunc func(*CgroupControl, *cgroups.Stats) error

const (
	cgroupRoot = "/sys/fs/cgroup"
	// CPU is the cpu controller.
	CPU = "cpu"
	// Memory is the memory controller.
	Memory = "memory"
	// Pids is the pids controller.
	Pids = "pids"
	// Blkio is the blkio controller.
	Blkio = "blkio"
)

var handlers = map[string]statFunc{
	CPU:    cpuStat,
	Memory: memoryStat,
	Pids:   pidsStat,
	Blkio:  blkioStat,
}

// AvailableControllers get string:bool map of all the available controllers.
func AvailableControllers() ([]string, error) {
	controllers := []string{}
	controllersFile := filepath.Join(cgroupRoot, "cgroup.controllers")

	// rootless cgroupv2: check available controllers for current user, systemd or servicescope will inherit
	if unshare.IsRootless() {
		userSlice, err := getCgroupPathForCurrentProcess()
		if err != nil {
			return controllers, err
		}
		// userSlice already contains '/' so not adding here
		basePath := cgroupRoot + userSlice
		controllersFile = filepath.Join(basePath, "cgroup.controllers")
	}
	controllersFileBytes, err := os.ReadFile(controllersFile)
	if err != nil {
		return nil, fmt.Errorf("failed while reading controllers for cgroup v2: %w", err)
	}
	for controllerName := range strings.FieldsSeq(string(controllersFileBytes)) {
		controllers = append(controllers, controllerName)
	}
	return controllers, nil
}

func getCgroupPathForCurrentProcess() (string, error) {
	path := fmt.Sprintf("/proc/%d/cgroup", os.Getpid())
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	cgroupPath := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		// set process cgroupPath only if entry is valid
		if _, p, ok := strings.Cut(text, "::"); ok {
			cgroupPath = p
		}
	}
	if err := s.Err(); err != nil {
		return cgroupPath, err
	}
	return cgroupPath, nil
}

// initialize initializes the specified hierarchy.
func (c *CgroupControl) initialize() (err error) {
	if err := createCgroupv2Path(filepath.Join(cgroupRoot, c.config.Path)); err != nil {
		return fmt.Errorf("creating cgroup path %s: %w", c.config.Path, err)
	}
	return nil
}

func readFileAsUint64(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v := cleanString(string(data))
	if v == "max" {
		return math.MaxUint64, nil
	}
	ret, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return ret, fmt.Errorf("parse %s from %s: %w", v, path, err)
	}
	return ret, nil
}

func readFileByKeyAsUint64(path, key string) (uint64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		k, v, _ := strings.Cut(line, " ")
		if k == key {
			v := cleanString(v)
			if v == "max" {
				return math.MaxUint64, nil
			}
			ret, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return ret, fmt.Errorf("parse %s from %s: %w", v, path, err)
			}
			return ret, nil
		}
	}

	return 0, fmt.Errorf("no key named %s from %s", key, path)
}

// New creates a new cgroup control.
func New(path string, resources *cgroups.Resources) (*CgroupControl, error) {
	_, err := IsCgroup2UnifiedMode()
	if err != nil {
		return nil, err
	}
	control := &CgroupControl{
		config: &cgroups.Cgroup{
			Path:      path,
			Resources: resources,
		},
	}

	if err := control.initialize(); err != nil {
		return nil, err
	}

	return control, nil
}

// NewSystemd creates a new cgroup control.
func NewSystemd(path string, resources *cgroups.Resources) (*CgroupControl, error) {
	_, err := IsCgroup2UnifiedMode()
	if err != nil {
		return nil, err
	}
	control := &CgroupControl{
		systemd: true,
		config: &cgroups.Cgroup{
			Path:      path,
			Resources: resources,
			Rootless:  unshare.IsRootless(),
		},
	}

	return control, nil
}

// Load loads an existing cgroup control.
func Load(path string) (*CgroupControl, error) {
	_, err := IsCgroup2UnifiedMode()
	if err != nil {
		return nil, err
	}
	control := &CgroupControl{
		systemd: false,
		config: &cgroups.Cgroup{
			Path: path,
		},
	}
	return control, nil
}

// CreateSystemdUnit creates the systemd cgroup.
func (c *CgroupControl) CreateSystemdUnit(path string) error {
	if !c.systemd {
		return errors.New("the cgroup controller is not using systemd")
	}

	conn, err := systemdDbus.NewWithContext(context.TODO())
	if err != nil {
		return err
	}
	defer conn.Close()

	return systemdCreate(c.config.Resources, path, conn)
}

// CreateSystemdUserUnit creates the systemd cgroup for the specified user.
func (c *CgroupControl) CreateSystemdUserUnit(path string, uid int) error {
	if !c.systemd {
		return errors.New("the cgroup controller is not using systemd")
	}

	conn, err := UserConnection(uid)
	if err != nil {
		return err
	}
	defer conn.Close()

	return systemdCreate(c.config.Resources, path, conn)
}

func dbusAuthConnection(uid int, createBus func(opts ...dbus.ConnOption) (*dbus.Conn, error)) (*dbus.Conn, error) {
	conn, err := createBus()
	if err != nil {
		return nil, err
	}

	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(uid))}

	err = conn.Auth(methods)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		return nil, err
	}

	return conn, nil
}

// Delete cleans a cgroup.
func (c *CgroupControl) Delete() error {
	return c.DeleteByPath(c.config.Path)
}

// DeleteByPathConn deletes the specified cgroup path using the specified
// dbus connection if needed.
func (c *CgroupControl) DeleteByPathConn(path string, conn *systemdDbus.Conn) error {
	if c.systemd {
		return systemdDestroyConn(path, conn)
	}
	return rmDirRecursively(filepath.Join(cgroupRoot, c.config.Path))
}

// DeleteByPath deletes the specified cgroup path.
func (c *CgroupControl) DeleteByPath(path string) error {
	if c.systemd {
		conn, err := systemdDbus.NewWithContext(context.TODO())
		if err != nil {
			return err
		}
		defer conn.Close()
		return c.DeleteByPathConn(path, conn)
	}
	return c.DeleteByPathConn(path, nil)
}

// Update updates the cgroups.
func (c *CgroupControl) Update(resources *cgroups.Resources) error {
	man, err := fs2.NewManager(c.config, filepath.Join(cgroupRoot, c.config.Path))
	if err != nil {
		return err
	}
	return man.Set(resources)
}

// AddPid moves the specified pid to the cgroup.
func (c *CgroupControl) AddPid(pid int) error {
	man, err := fs2.NewManager(c.config, filepath.Join(cgroupRoot, c.config.Path))
	if err != nil {
		return err
	}
	return man.Apply(pid)
}

// Stat returns usage statistics for the cgroup.
func (c *CgroupControl) Stat() (*cgroups.Stats, error) {
	m := cgroups.Stats{}
	found := false
	for _, statFunc := range handlers {
		if err := statFunc(c, &m); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			continue
		}
		found = true
	}
	if !found {
		return nil, ErrStatCgroup
	}
	return &m, nil
}

func readCgroupMapPath(path string) (map[string][]string, error) {
	ret := map[string][]string{}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ret, nil
		}
		return nil, fmt.Errorf("open file %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ret[parts[0]] = parts[1:]
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", path, err)
	}
	return ret, nil
}

func readCgroup2MapFile(ctr *CgroupControl, name string) (map[string][]string, error) {
	p := filepath.Join(cgroupRoot, ctr.config.Path, name)

	return readCgroupMapPath(p)
}

var TestMode bool

func createCgroupv2Path(path string) (deferredError error) {
	if !strings.HasPrefix(path, cgroupRoot+"/") {
		return fmt.Errorf("invalid cgroup path %s", path)
	}
	content, err := os.ReadFile(filepath.Join(cgroupRoot, "cgroup.controllers"))
	if err != nil {
		return err
	}
	ctrs := bytes.Fields(content)
	res := append([]byte("+"), bytes.Join(ctrs, []byte(" +"))...)

	current := "/sys/fs"
	elements := strings.Split(path, "/")
	for i, e := range elements[3:] {
		current = filepath.Join(current, e)
		if i > 0 {
			if err := os.Mkdir(current, 0o755); err != nil {
				if !os.IsExist(err) {
					return err
				}
			} else {
				// If the directory was created, be sure it is not left around on errors.
				defer func() {
					if deferredError != nil {
						os.Remove(current)
					}
				}()
			}
		}
		// We enable the controllers for all the path components except the last one.  It is not allowed to add
		// PIDs if there are already enabled controllers.
		if i < len(elements[3:])-1 {
			subtreeControl := filepath.Join(current, "cgroup.subtree_control")
			if err := os.WriteFile(subtreeControl, res, 0o755); err != nil {
				// The kernel returns ENOENT either if the file itself is missing, or a controller
				if errors.Is(err, os.ErrNotExist) {
					if err2 := fileutils.Exists(subtreeControl); err2 != nil {
						// If the file itself is missing, return the original error.
						return err
					}
					repeatAttempts := 1000
					for repeatAttempts > 0 {
						// store the controllers that failed to be enabled, so we can retry them
						newCtrs := [][]byte{}
						for _, ctr := range ctrs {
							// Try to enable each controller individually, at least we can give a better error message if any fails.
							if err := os.WriteFile(subtreeControl, fmt.Appendf(nil, "+%s\n", ctr), 0o755); err != nil {
								// The kernel can return EBUSY when a process was moved to a sub-cgroup
								// and the controllers are enabled in its parent cgroup.  Retry a few times when
								// it happens.
								if errors.Is(err, unix.EBUSY) {
									newCtrs = append(newCtrs, ctr)
								} else {
									return fmt.Errorf("enabling controller %s: %w", ctr, err)
								}
							}
						}
						if len(newCtrs) == 0 {
							err = nil
							break
						}
						ctrs = newCtrs
						repeatAttempts--
						time.Sleep(time.Millisecond)
					}
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func cleanString(s string) string {
	return strings.Trim(s, "\n")
}

// SystemCPUUsage returns the system usage for all the cgroups.
func SystemCPUUsage() (uint64, error) {
	_, err := IsCgroup2UnifiedMode()
	if err != nil {
		return 0, err
	}
	files, err := os.ReadDir(cgroupRoot)
	if err != nil {
		return 0, err
	}
	var total uint64
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		p := filepath.Join(cgroupRoot, file.Name(), "cpu.stat")

		values, err := readCgroupMapPath(p)
		if err != nil {
			return 0, err
		}

		if val, found := values["usage_usec"]; found {
			v, err := strconv.ParseUint(cleanString(val[0]), 10, 64)
			if err != nil {
				return 0, err
			}
			total += v * 1000
		}
	}
	return total, nil
}

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 cgroup2 mode.
func IsCgroup2UnifiedMode() (bool, error) {
	isUnifiedOnce.Do(func() {
		var st syscall.Statfs_t
		if err := syscall.Statfs("/sys/fs/cgroup", &st); err != nil {
			isUnified, isUnifiedErr = false, err
		} else {
			isUnified, isUnifiedErr = st.Type == unix.CGROUP2_SUPER_MAGIC, nil
		}
	})
	return isUnified, isUnifiedErr
}

// UserConnection returns an user connection to D-BUS.
func UserConnection(uid int) (*systemdDbus.Conn, error) {
	return systemdDbus.NewConnection(func() (*dbus.Conn, error) {
		return dbusAuthConnection(uid, dbus.SessionBusPrivateNoAutoStartup)
	})
}

// UserOwnsCurrentSystemdCgroup checks whether the current EUID owns the
// current cgroup.
func UserOwnsCurrentSystemdCgroup() (bool, error) {
	uid := os.Geteuid()

	_, err := IsCgroup2UnifiedMode()
	if err != nil {
		return false, err
	}

	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)

		if len(parts) < 3 {
			continue
		}

		// If we are on a cgroup v2 system and there are cgroup v1 controllers
		// mounted, ignore them when the current process is at the root cgroup.
		if parts[1] != "" && parts[2] == "/" {
			continue
		}

		cgroupPath := filepath.Join(cgroupRoot, parts[2])

		st, err := os.Stat(cgroupPath)
		if err != nil {
			return false, err
		}

		s := st.Sys()
		if s == nil {
			return false, fmt.Errorf("stat cgroup path is nil %s", cgroupPath)
		}

		//nolint:errcheck // This cast should never fail, if it does we get a interface
		// conversion panic and a stack trace on how we ended up here which is more
		// valuable than returning a human friendly error test as we don't know how it
		// happened.
		if int(s.(*syscall.Stat_t).Uid) != uid {
			return false, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("parsing file /proc/self/cgroup: %w", err)
	}
	return true, nil
}

// rmDirRecursively delete recursively a cgroup directory.
// It differs from os.RemoveAll as it doesn't attempt to unlink files.
// On cgroupfs we are allowed only to rmdir empty directories.
func rmDirRecursively(path string) error {
	killProcesses := func(signal syscall.Signal) {
		if signal == unix.SIGKILL {
			if err := os.WriteFile(filepath.Join(path, "cgroup.kill"), []byte("1"), 0o600); err == nil {
				return
			}
		}
		// kill all the processes that are still part of the cgroup
		if procs, err := os.ReadFile(filepath.Join(path, "cgroup.procs")); err == nil {
			for pidS := range strings.SplitSeq(string(procs), "\n") {
				if pid, err := strconv.Atoi(pidS); err == nil {
					_ = unix.Kill(pid, signal)
				}
			}
		}
	}

	if err := os.Remove(path); err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, i := range entries {
		if i.IsDir() {
			if err := rmDirRecursively(filepath.Join(path, i.Name())); err != nil {
				return err
			}
		}
	}

	attempts := 0
	for {
		err := os.Remove(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if errors.Is(err, unix.EBUSY) {
			// send a SIGTERM after 3 second
			if attempts == 300 {
				killProcesses(unix.SIGTERM)
			}
			// send SIGKILL after 8 seconds
			if attempts == 800 {
				killProcesses(unix.SIGKILL)
			}
			// give up after 10 seconds
			if attempts < 1000 {
				time.Sleep(time.Millisecond * 10)
				attempts++
				continue
			}
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
}
