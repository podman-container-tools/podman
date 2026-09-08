//go:build !remote

package generate

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/fileutils"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"tags.cncf.io/container-device-interface/pkg/cdi"
)

// DevicesFromPath computes a list of devices
func DevicesFromPath(g *generate.Generator, devicePath string, config *config.Config) error {
	if isCDIDevice(devicePath) {
		registry, err := cdi.NewCache(
			cdi.WithSpecDirs(config.Engine.CdiSpecDirs.Get()...),
			cdi.WithAutoRefresh(false),
		)
		if err != nil {
			return fmt.Errorf("creating CDI registry: %w", err)
		}
		if err := registry.Refresh(); err != nil {
			logrus.Debugf("The following error was triggered when refreshing the CDI registry: %v", err)
		}
		if _, err := registry.InjectDevices(g.Config, devicePath); err != nil {
			return fmt.Errorf("setting up CDI devices: %w", err)
		}
		return nil
	}
	warnedGIDs := make(map[int]bool)
	devs := strings.Split(devicePath, ":")
	resolvedDevicePath := devs[0]
	// check if it is a symbolic link
	if src, err := os.Lstat(resolvedDevicePath); err == nil && src.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkedPathOnHost, err := filepath.EvalSymlinks(resolvedDevicePath); err == nil {
			resolvedDevicePath = linkedPathOnHost
		}
	}
	st, err := os.Stat(resolvedDevicePath)
	if err != nil {
		return err
	}
	if st.IsDir() {
		found := false
		src := resolvedDevicePath
		dest := src
		var devmode string
		if len(devs) > 1 {
			if len(devs[1]) > 0 && devs[1][0] == '/' {
				dest = devs[1]
			} else {
				devmode = devs[1]
			}
		}
		if len(devs) > 2 {
			if devmode != "" {
				return fmt.Errorf("invalid device specification %s: %w", devicePath, unix.EINVAL)
			}
			devmode = devs[2]
		}

		// mount the internal devices recursively
		if err := filepath.WalkDir(resolvedDevicePath, func(dpath string, d fs.DirEntry, _ error) error {
			if d.Type()&os.ModeDevice == os.ModeDevice {
				found = true
				device := fmt.Sprintf("%s:%s", dpath, filepath.Join(dest, strings.TrimPrefix(dpath, src)))
				if devmode != "" {
					device = fmt.Sprintf("%s:%s", device, devmode)
				}
				if err := addDevice(g, device, warnedGIDs); err != nil {
					return fmt.Errorf("failed to add %s device: %w", dpath, err)
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("no devices found in %s: %w", devicePath, unix.EINVAL)
		}
		return nil
	}
	return addDevice(g, strings.Join(append([]string{resolvedDevicePath}, devs[1:]...), ":"), warnedGIDs)
}

func BlockAccessToKernelFilesystems(privileged, pidModeIsHost bool, mask, unmask []string, g *generate.Generator) {
	if !privileged {
		for _, mp := range config.DefaultMaskedPaths() {
			// check that the path to mask is not in the list of paths to unmask
			if shouldMask(mp, unmask) {
				g.AddLinuxMaskedPaths(mp)
			}
		}
		for _, rp := range config.DefaultReadOnlyPaths {
			if shouldMask(rp, unmask) {
				g.AddLinuxReadonlyPaths(rp)
			}
		}

		if pidModeIsHost && rootless.IsRootless() {
			return
		}
	}

	// mask the paths provided by the user
	for _, mp := range mask {
		if !path.IsAbs(mp) && mp != "" {
			logrus.Errorf("Path %q is not an absolute path, skipping...", mp)
			continue
		}
		g.AddLinuxMaskedPaths(mp)
	}
}

// findUnmappedDeviceGID finds the host GID of a device whose GID is not
// mapped in the current user namespace (appears as overflowGID).
// It reads the real host GID from the device_gids file saved before re-exec.
func getDeviceHostGID(devicePath string) int {
	uid := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uid == "" {
		return -1
	}
	deviceGIDsFile := fmt.Sprintf("/run/user/%s/libpod/tmp/device_gids", uid)
	content, err := os.ReadFile(deviceGIDsFile)
	if err != nil {
		return -1
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(content)), "\n") {
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		path := line[:idx]
		gidStr := line[idx+1:]
		if path == devicePath {
			gid, err := strconv.Atoi(gidStr)
			if err == nil {
				logrus.Debugf("getDeviceHostGID: %s → host GID %d", devicePath, gid)
				return gid
			}
		}
	}
	return -1
}

// containerGIDToHostGID converts a container-namespace GID to the real host GID
// by reading /proc/self/gid_map and doing a reverse lookup.
// Format of /proc/self/gid_map: containerID hostID size
func containerGIDToHostGID(containerGID int) int {
	content, err := os.ReadFile("/proc/self/gid_map")
	if err != nil {
		return containerGID
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(content)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		cStart, err1 := strconv.Atoi(fields[0])
		hStart, err2 := strconv.Atoi(fields[1])
		size, err3 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		// Check if containerGID falls in this range
		if containerGID >= cStart && containerGID < cStart+size {
			// Reverse map: hostGID = hStart + (containerGID - cStart)
			hostGID := hStart + (containerGID - cStart)
			logrus.Debugf("containerGIDToHostGID: container GID %d → host GID %d", containerGID, hostGID)
			return hostGID
		}
	}
	// No mapping found — return original
	return containerGID
}

// overflowGID returns the overflow GID used by the kernel when a GID
// has no mapping in the current user namespace.
func overflowGID() int {
	content, err := os.ReadFile("/proc/sys/kernel/overflowgid")
	if err != nil {
		return 65534 // default
	}
	gid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 65534
	}
	return gid
}

// getHostGroups reads the host supplementary groups that were saved
// before entering the user namespace. After re-exec, os.Getgroups()
// returns namespace-mapped groups, not real host groups.
func getHostGroups() []int {
	// Try reading from state dir file first
	// stateDir is typically /run/user/<uid>/libpod
	uid := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uid == "" {
		// Not in rootless mode or env not set
		groups, _ := os.Getgroups()
		return groups
	}

	// Common state dir locations
	// stateDir in rootless_linux.go is runtimeDir/libpod/tmp
	stateDirs := []string{
		fmt.Sprintf("/run/user/%s/libpod/tmp/host_groups", uid),
		fmt.Sprintf("/run/user/%s/libpod/host_groups", uid),
	}

	for _, path := range stateDirs {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var groups []int
		for s := range strings.SplitSeq(strings.TrimSpace(string(content)), ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			g, err := strconv.Atoi(s)
			if err != nil {
				continue
			}
			groups = append(groups, g)
		}
		if len(groups) > 0 {
			logrus.Debugf("Read host groups from %s: %v", path, groups)
			return groups
		}
	}

	// Fallback
	groups, _ := os.Getgroups()
	return groups
}

// deviceGIDMissingFromSubgid checks if the device's owning GID is accessible
// to the current rootless user. If the user is a member of the device's group
// on the host but that GID is not in /etc/subgid, it logs a clear warning
// explaining exactly how to fix the issue.
//
// Returns true if the GID is already mapped (no action needed),
// false if it is missing and the user should be warned.
func deviceGIDMissingFromSubgid(src string, warnedGIDs map[int]bool) bool {
	// stat the device to get its GID
	var st syscall.Stat_t
	if err := syscall.Stat(src, &st); err != nil {
		// cannot stat let it proceed, error will surface later
		return true
	}
	deviceGID := int(st.Gid)

	// The device GID we got from stat() is a container-namespace GID.
	// We need the real host GID for comparison with host groups.
	// If the GID is overflowGID (65534), it means the device's real host GID
	// has NO mapping in this user namespace — we cannot determine it from
	// the namespace alone. In this case, find it from host_groups by exclusion.
	// Otherwise, reverse-map container GID → host GID via /proc/self/gid_map.
	ovGID := overflowGID()
	if deviceGID == ovGID {
		// Device GID is unmapped in this namespace.
		// Read real host GID from file saved before re-exec.
		realGID := getDeviceHostGID(src)
		if realGID >= 0 {
			logrus.Debugf("deviceGIDMissingFromSubgid: overflow GID, real host GID=%d", realGID)
			deviceGID = realGID
		} else {
			logrus.Debugf("deviceGIDMissingFromSubgid: overflow GID but no saved GID for %s", src)
			return true
		}
	} else {
		deviceGID = containerGIDToHostGID(deviceGID)
	}

	// Deduplicate warnings — if we already warned about this GID
	// (e.g. directory with many devices sharing the same group), skip.
	if warnedGIDs[deviceGID] {
		return false
	}
	warnedGIDs[deviceGID] = true

	// Get host groups saved before entering user namespace.
	// os.Getgroups() cannot be used here because addDevice runs inside
	// the user namespace where groups are remapped.
	hostGroups := getHostGroups()
	if len(hostGroups) == 0 {
		return true
	}
	groupName := fmt.Sprintf("GID %d", deviceGID)
	if grp, err := user.LookupGroupId(fmt.Sprintf("%d", deviceGID)); err == nil {
		groupName = grp.Name
	}
	deviceName := filepath.Base(src)
	username := os.Getenv("USER")
	if username == "" {
		if u, err := user.LookupId(fmt.Sprintf("%d", os.Getuid())); err == nil {
			username = u.Username
		}
	}

	// Check if user is a member of the device group on the host
	userIsMember := slices.Contains(hostGroups, deviceGID)

	if !userIsMember {
		// User is not even a member of this group on the host.
		// Warn them that  Podman cannot fix this automatically.
		msg := fmt.Sprintf("Device %s is owned by group '%s' (GID %d).\n"+
			"You are not a member of this group on the host.\n"+
			"Access will be denied inside the container.\n"+
			"To fix run:\n"+
			"sudo usermod -aG %s %s\n"+
			"Then log out and log back in.",
			deviceName, groupName, deviceGID, groupName, username)
		logrus.Warn(msg)
		return false
	}

	// User IS a member check /etc/subgid
	if isGIDInSubgid(username, deviceGID, "/etc/subgid") {
		return true
	}
	// GID is not in subgid , warn with exact fix command
	msg := fmt.Sprintf("Device %s is owned by group '%s' (GID %d).\n"+
		"You are a member of this group on the host, but GID %d is not in /etc/subgid.\n"+
		"The device will appear as 'nobody:nobody' inside the container and access will be denied.\n"+
		"To fix this, run:\n"+
		"echo \"%s:%d:1\" | sudo tee -a /etc/subgid && podman system migrate",
		deviceName, groupName, deviceGID, deviceGID, username, deviceGID)
	logrus.Warn(msg)
	return false
}

func addDevice(g *generate.Generator, device string, warnedGIDs map[int]bool) error {
	src, dst, permissions, err := ParseDevice(device)
	if err != nil {
		return err
	}
	dev, err := util.DeviceFromPath(src)
	if err != nil {
		return fmt.Errorf("%s is not a valid device: %w", src, err)
	}
	if rootless.IsRootless() {
		if err := fileutils.Exists(src); err != nil {
			return err
		}
		// Check device GID mapping and warn user if access will fail
		// This runs only when --device is used in rootless mode
		deviceGIDMissingFromSubgid(src, warnedGIDs)
		perm := "ro"
		if strings.Contains(permissions, "w") {
			perm = "rw"
		}
		devMnt := spec.Mount{
			Destination: dst,
			Type:        define.TypeBind,
			Source:      src,
			Options:     []string{"slave", "nosuid", "noexec", perm, "rbind"},
		}
		g.Config.Mounts = append(g.Config.Mounts, devMnt)
		return nil
	} else if src == "/dev/fuse" {
		// if the user is asking for fuse inside the container
		// make sure the module is loaded.
		f, err := unix.Open(src, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err == nil {
			unix.Close(f)
		}
	}
	dev.Path = dst
	g.AddDevice(*dev)
	g.AddLinuxResourcesDevice(true, dev.Type, &dev.Major, &dev.Minor, permissions)
	return nil
}

func supportAmbientCapabilities() bool {
	err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_IS_SET, 0, 0, 0)
	return err == nil
}

func shouldMask(mask string, unmask []string) bool {
	for _, m := range unmask {
		if strings.EqualFold(m, "all") {
			return false
		}
		for m1 := range strings.SplitSeq(m, ":") {
			match, err := filepath.Match(m1, mask)
			if err != nil {
				logrus.Error(err.Error())
			}
			if match {
				return false
			}
		}
	}
	return true
}

// isGIDInSubgid checks whether the given GID is covered by any subgid
// range for username in the given file (normally /etc/subgid).
// Accepting the path as a parameter makes the function testable.
func isGIDInSubgid(username string, gid int, subgidFile string) bool {
	content, err := os.ReadFile(subgidFile)
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		parts := strings.Split(strings.TrimSpace(line), ":")
		if len(parts) != 3 {
			continue
		}
		if parts[0] != username {
			continue
		}
		start, err1 := strconv.Atoi(parts[1])
		count, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			continue
		}
		if gid >= start && gid < start+count {
			return true
		}
	}
	return false
}
