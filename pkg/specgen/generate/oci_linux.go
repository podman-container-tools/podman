//go:build !remote

package generate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/specgen"
	"golang.org/x/sys/unix"
)

const devMqueue = "/dev/mqueue"

var isMqueueSupported = sync.OnceValue(func() bool {
	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 && fields[len(fields)-1] == "mqueue" {
			return true
		}
	}
	if err := scanner.Err(); err != nil {
		logrus.Warnf("Failed to read /proc/filesystems: %v, assuming mqueue is not supported", err)
	}
	return false
})

func setProcOpts(s *specgen.SpecGenerator, g *generate.Generator) {
	if s.ProcOpts == nil {
		return
	}
	for i := range g.Config.Mounts {
		if g.Config.Mounts[i].Destination == "/proc" {
			g.Config.Mounts[i].Options = s.ProcOpts
			return
		}
	}
}

func setDevOptsReadOnly(g *generate.Generator) {
	for i := range g.Config.Mounts {
		if g.Config.Mounts[i].Destination == "/dev" {
			g.Config.Mounts[i].Options = append(g.Config.Mounts[i].Options, "ro")
			return
		}
	}
}

// canMountSys is a best-effort heuristic to detect whether mounting a new sysfs is permitted in the container
func canMountSys(isRootless, isNewUserns bool, s *specgen.SpecGenerator) bool {
	if s.NetNS.IsHost() && (isRootless || isNewUserns) {
		return false
	}
	if isNewUserns {
		switch s.NetNS.NSMode {
		case specgen.Pasta, specgen.Private, specgen.NoNetwork, specgen.Bridge:
			return true
		default:
			return false
		}
	}
	return true
}

func getCgroupPermissions(unmask []string) string {
	ro := "ro"
	rw := "rw"
	cgroup := "/sys/fs/cgroup"

	if len(unmask) != 0 && unmask[0] == "ALL" {
		return rw
	}

	for _, p := range unmask {
		if path.Clean(p) == cgroup {
			return rw
		}
	}
	return ro
}

// SpecGenToOCI returns the base configuration for the container.
func SpecGenToOCI(_ context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, newImage *libimage.Image, mounts []spec.Mount, pod *libpod.Pod, finalCmd []string, compatibleOptions *libpod.InfraInherit) (*spec.Spec, error) {
	cgroupPerm := getCgroupPermissions(s.Unmask)

	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")
	g.HostSpecific = true
	addCgroup := true

	isRootless := rootless.IsRootless()
	isNewUserns := s.UserNS.IsContainer() || s.UserNS.IsPath() || s.UserNS.IsPrivate() || s.UserNS.IsPod() || s.UserNS.IsAuto()

	canMountSys := canMountSys(isRootless, isNewUserns, s)

	if s.IsPrivileged() && canMountSys {
		cgroupPerm = "rw"
		g.RemoveMount("/sys")
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "rw"},
		}
		g.AddMount(sysMnt)
	}
	if !canMountSys {
		addCgroup = false
		g.RemoveMount("/sys")
		r := "ro"
		if s.IsPrivileged() {
			r = "rw"
		}
		sysMnt := spec.Mount{
			Destination: "/sys",
			Type:        define.TypeBind,
			Source:      "/sys",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", r, "rbind"},
		}
		g.AddMount(sysMnt)
		g.RemoveMount("/sys/fs/cgroup")

		sysFsCgroupMnt := spec.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "/sys/fs/cgroup",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", r},
		}
		g.AddMount(sysFsCgroupMnt)
		if !s.IsPrivileged() && isRootless {
			g.AddLinuxMaskedPaths("/sys/kernel")
		}
	}
	gid5Available := true
	if isRootless {
		nGids, err := rootless.GetAvailableGids()
		if err != nil {
			return nil, err
		}
		gid5Available = nGids >= 5
	}
	// When using a different user namespace, check that the GID 5 is mapped inside
	// the container.
	if gid5Available && (s.IDMappings != nil && len(s.IDMappings.GIDMap) > 0) {
		mappingFound := false
		for _, r := range s.IDMappings.GIDMap {
			if r.ContainerID <= 5 && 5 < r.ContainerID+r.Size {
				mappingFound = true
				break
			}
		}
		if !mappingFound {
			gid5Available = false
		}
	}
	if !gid5Available {
		// If we have no GID mappings, the gid=5 default option would fail, so drop it.
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        define.TypeDevpts,
			Source:      define.TypeDevpts,
			Options:     []string{"rprivate", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}

	inUserNS := isRootless || isNewUserns

	// Check /proc/filesystems to see if the kernel supports mqueue.
	if !isMqueueSupported() {
		g.RemoveMount(devMqueue)
	} else if inUserNS && s.IpcNS.IsHost() {
		g.RemoveMount(devMqueue)
		devMqueue := spec.Mount{
			Destination: devMqueue,
			Type:        define.TypeBind,
			Source:      devMqueue,
			Options:     []string{define.TypeBind, "nosuid", "noexec", "nodev"},
		}
		g.AddMount(devMqueue)
	}
	if inUserNS && s.PidNS.IsHost() {
		g.RemoveMount("/proc")
		procMount := spec.Mount{
			Destination: "/proc",
			Type:        define.TypeBind,
			Source:      "/proc",
			Options:     []string{"rbind", "nosuid", "noexec", "nodev"},
		}
		g.AddMount(procMount)
	}

	if addCgroup {
		cgroupMnt := spec.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "relatime", cgroupPerm},
		}
		g.AddMount(cgroupMnt)
	}

	g.Config.Linux.Personality = s.Personality

	g.SetProcessCwd(s.WorkDir)

	g.SetProcessArgs(finalCmd)

	if s.Terminal != nil {
		g.SetProcessTerminal(*s.Terminal)
	}

	for key, val := range s.Annotations {
		g.AddAnnotation(key, val)
	}

	if s.IntelRdt != nil {
		if s.IntelRdt.ClosID != "" {
			g.SetLinuxIntelRdtClosID(s.IntelRdt.ClosID)
		}
	}

	if s.ResourceLimits != nil {
		out, err := json.Marshal(s.ResourceLimits)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(out, g.Config.Linux.Resources)
		if err != nil {
			return nil, err
		}
		g.Config.Linux.Resources = s.ResourceLimits
	}

	weightDevices, err := WeightDevices(s.WeightDevice)
	if err != nil {
		return nil, err
	}
	if len(weightDevices) > 0 {
		for _, dev := range weightDevices {
			g.AddLinuxResourcesBlockIOWeightDevice(dev.Major, dev.Minor, *dev.Weight)
		}
	}

	// Devices
	// set the default rule at the beginning of device configuration
	if !inUserNS && !s.IsPrivileged() {
		g.AddLinuxResourcesDevice(false, "", nil, nil, "rwm")
	}

	var userDevices []spec.LinuxDevice
	// add default devices from containers.conf
	for _, device := range rtc.Containers.Devices.Get() {
		if err = DevicesFromPath(&g, device, rtc); err != nil {
			return nil, err
		}
	}
	if len(compatibleOptions.HostDeviceList) > 0 && len(s.Devices) == 0 {
		userDevices = compatibleOptions.HostDeviceList
	} else {
		userDevices = s.Devices
	}
	// add default devices specified by caller
	for _, device := range userDevices {
		if err = DevicesFromPath(&g, device.Path, rtc); err != nil {
			return nil, err
		}
	}
	s.HostDeviceList = userDevices

	// set the devices cgroup when not running in a user namespace
	if isRootless && len(s.DeviceCgroupRule) > 0 {
		return nil, fmt.Errorf("device cgroup rules are not supported in rootless mode or in a user namespace")
	}
	if !isRootless && !s.IsPrivileged() {
		for _, dev := range s.DeviceCgroupRule {
			g.AddLinuxResourcesDevice(true, dev.Type, dev.Major, dev.Minor, dev.Access)
		}
	}

	BlockAccessToKernelFilesystems(s.IsPrivileged(), s.PidNS.IsHost(), s.Mask, s.Unmask, &g)

	g.ClearProcessEnv()
	for name, val := range s.Env {
		g.AddProcessEnv(name, val)
	}

	addRlimits(s, &g)

	// NAMESPACES
	if err := specConfigureNamespaces(s, &g, rt, pod); err != nil {
		return nil, err
	}
	configSpec := g.Config

	if err := securityConfigureGenerator(s, &g, newImage, rtc); err != nil {
		return nil, err
	}

	// BIND MOUNTS
	configSpec.Mounts = SupersedeUserMounts(mounts, configSpec.Mounts)
	// Process mounts to ensure correct options
	if err := InitFSMounts(configSpec.Mounts); err != nil {
		return nil, err
	}

	// Add annotations
	if configSpec.Annotations == nil {
		configSpec.Annotations = make(map[string]string)
	}

	if s.Remove != nil && *s.Remove {
		configSpec.Annotations[define.InspectAnnotationAutoremove] = define.InspectResponseTrue
	}

	if s.RemoveImage != nil && *s.RemoveImage {
		configSpec.Annotations[define.InspectAnnotationAutoremoveImage] = define.InspectResponseTrue
	}

	if len(s.VolumesFrom) > 0 {
		configSpec.Annotations[define.VolumesFromAnnotation] = strings.Join(s.VolumesFrom, ";")
	}

	if s.IsPrivileged() {
		configSpec.Annotations[define.InspectAnnotationPrivileged] = define.InspectResponseTrue
	}

	if s.Init != nil && *s.Init {
		configSpec.Annotations[define.InspectAnnotationInit] = define.InspectResponseTrue
	}

	if s.OOMScoreAdj != nil {
		g.SetProcessOOMScoreAdj(*s.OOMScoreAdj)
	}

	setProcOpts(s, &g)
	roFS := false
	if s.ReadOnlyFilesystem != nil {
		roFS = *s.ReadOnlyFilesystem
	}
	rwTmpfs := false
	if s.ReadWriteTmpfs != nil {
		rwTmpfs = *s.ReadWriteTmpfs
	}
	if roFS && !rwTmpfs {
		setDevOptsReadOnly(&g)
	}

	return configSpec, nil
}

func WeightDevices(wtDevices map[string]spec.LinuxWeightDevice) ([]spec.LinuxWeightDevice, error) {
	devs := []spec.LinuxWeightDevice{}
	for k, v := range wtDevices {
		statT := unix.Stat_t{}
		if err := unix.Stat(k, &statT); err != nil {
			return nil, fmt.Errorf("failed to inspect '%s' in --blkio-weight-device: %w", k, err)
		}
		dev := new(spec.LinuxWeightDevice)
		dev.Major = (int64(unix.Major(uint64(statT.Rdev)))) //nolint: unconvert
		dev.Minor = (int64(unix.Minor(uint64(statT.Rdev)))) //nolint: unconvert
		dev.Weight = v.Weight
		devs = append(devs, *dev)
	}
	return devs, nil
}
