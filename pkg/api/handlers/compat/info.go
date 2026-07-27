//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"
	"net/netip"
	"os"
	goRuntime "runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/swarm"
	dockerSystem "github.com/moby/moby/api/types/system"
	"github.com/opencontainers/selinux/go-selinux"
	log "github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/sysinfo"
	"go.podman.io/image/v5/pkg/sysregistriesv2"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/rootless"
)

// ociRuntimeFeaturesKey is the key used for an OCI runtime's
// status: features.
// Described in https://github.com/opencontainers/runtime-spec/blob/main/features.md
const ociRuntimeFeaturesKey = "org.opencontainers.runtime-spec.features"

func GetInfo(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 internal
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	infoData, err := runtime.Info()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to obtain system memory info: %w", err))
		return
	}

	configInfo, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to obtain runtime config: %w", err))
		return
	}
	versionInfo, err := define.GetVersion()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to obtain podman versions: %w", err))
		return
	}
	stateInfo := getContainersState(runtime)
	sysInfo := sysinfo.New(true)

	// FIXME: Need to expose if runtime supports Checkpointing
	// liveRestoreEnabled := criu.CheckForCriu() && configInfo.RuntimeSupportsCheckpoint()
	info := &handlers.Info{
		Info: dockerSystem.Info{
			Architecture:        goRuntime.GOARCH,
			CPUCfsPeriod:        sysInfo.CPUCfsPeriod,
			CPUCfsQuota:         sysInfo.CPUCfsQuota,
			CPUSet:              sysInfo.Cpuset,
			CPUShares:           sysInfo.CPUShares,
			CgroupDriver:        getCgroupDriver(configInfo.Engine.CgroupManager, rootless.IsRootless()),
			CDISpecDirs:         infoData.Host.CDISpecDirs,
			ContainerdCommit:    dockerSystem.Commit{},
			Containers:          infoData.Store.ContainerStore.Number,
			ContainersPaused:    stateInfo[define.ContainerStatePaused],
			ContainersRunning:   stateInfo[define.ContainerStateRunning],
			ContainersStopped:   stateInfo[define.ContainerStateStopped] + stateInfo[define.ContainerStateExited],
			Debug:               log.IsLevelEnabled(log.DebugLevel),
			DefaultAddressPools: getDefaultAddressPools(configInfo),
			DefaultRuntime:      configInfo.Engine.OCIRuntime,
			DiscoveredDevices:   getDiscoveredDevices(infoData.Host.DiscoveredDevices),
			DockerRootDir:       infoData.Store.GraphRoot,
			Driver:              infoData.Store.GraphDriverName,
			DriverStatus:        getGraphStatus(infoData.Store.GraphStatus),
			ExperimentalBuild:   true,
			GenericResources:    nil,
			HTTPProxy:           getEnv("http_proxy"),
			HTTPSProxy:          getEnv("https_proxy"),
			ID:                  uuid.New().String(),
			IPv4Forwarding:      !sysInfo.IPv4ForwardingDisabled,
			Images:              infoData.Store.ImageStore.Number,
			IndexServerAddress:  "",
			InitBinary:          "",
			InitCommit:          dockerSystem.Commit{},
			Isolation:           "",
			KernelVersion:       infoData.Host.Kernel,
			Labels:              nil,
			LiveRestoreEnabled:  false,
			LoggingDriver:       "",
			MemTotal:            infoData.Host.MemTotal,
			MemoryLimit:         sysInfo.MemoryLimit,
			NCPU:                goRuntime.NumCPU(),
			NEventsListener:     0,
			NFd:                 getFdCount(),
			NGoroutines:         goRuntime.NumGoroutine(),
			Name:                infoData.Host.Hostname,
			NoProxy:             getEnv("no_proxy"),
			OSType:              goRuntime.GOOS,
			OSVersion:           infoData.Host.Distribution.Version,
			OomKillDisable:      sysInfo.OomKillDisable,
			OperatingSystem:     infoData.Host.Distribution.Distribution,
			PidsLimit:           sysInfo.PidsLimit,
			Plugins: dockerSystem.PluginsInfo{
				Volume:  infoData.Plugins.Volume,
				Network: infoData.Plugins.Network,
				Log:     infoData.Plugins.Log,
			},
			ProductLicense:  "Apache-2.0",
			RegistryConfig:  getServiceConfig(runtime),
			RuncCommit:      dockerSystem.Commit{},
			Runtimes:        getRuntimes(runtime, configInfo),
			SecurityOptions: getSecOpts(sysInfo, configInfo),
			ServerVersion:   versionInfo.Version,
			SwapLimit:       sysInfo.SwapLimit,
			Swarm: swarm.Info{
				LocalNodeState: swarm.LocalNodeStateInactive,
			},
			SystemStatus: nil,
			SystemTime:   time.Now().Format(time.RFC3339Nano),
			Warnings:     []string{},
		},
		BuildahVersion:     infoData.Host.BuildahVersion,
		CPURealtimePeriod:  sysInfo.CPURealtimePeriod,
		CPURealtimeRuntime: sysInfo.CPURealtimeRuntime,
		CgroupVersion:      strings.TrimPrefix(infoData.Host.CgroupsVersion, "v"),
		Rootless:           rootless.IsRootless(),
		SwapFree:           infoData.Host.SwapFree,
		SwapTotal:          infoData.Host.SwapTotal,
		Uptime:             infoData.Host.Uptime,
	}
	// The Status field on runtimes was introduced in the Docker API v1.44.
	if _, err := utils.SupportedVersion(r, "<1.44.0"); err == nil {
		for k, rt := range info.Runtimes {
			info.Runtimes[k] = dockerSystem.RuntimeWithStatus{Runtime: rt.Runtime}
		}
	}

	utils.WriteResponse(w, http.StatusOK, info)
}

func getDiscoveredDevices(discoveredDevices []define.DeviceInfo) []dockerSystem.DeviceInfo {
	devices := make([]dockerSystem.DeviceInfo, 0, len(discoveredDevices))
	for _, device := range discoveredDevices {
		devices = append(devices, dockerSystem.DeviceInfo{
			Source: device.Source,
			ID:     device.ID,
		})
	}
	return devices
}

func getServiceConfig(runtime *libpod.Runtime) *registry.ServiceConfig {
	var indexConfs map[string]*registry.IndexInfo

	regs, err := sysregistriesv2.GetRegistries(runtime.SystemContext())
	if err == nil {
		indexConfs = make(map[string]*registry.IndexInfo, len(regs))
		for _, reg := range regs {
			mirrors := make([]string, len(reg.Mirrors))
			for i, mirror := range reg.Mirrors {
				mirrors[i] = mirror.Location
			}
			indexConfs[reg.Prefix] = &registry.IndexInfo{
				Name:    reg.Prefix,
				Mirrors: mirrors,
				Secure:  !reg.Insecure,
			}
		}
	} else {
		log.Warnf("failed to get registries configuration: %v", err)
		indexConfs = make(map[string]*registry.IndexInfo)
	}

	return &registry.ServiceConfig{
		InsecureRegistryCIDRs: make([]netip.Prefix, 0),
		IndexConfigs:          indexConfs,
		Mirrors:               make([]string, 0),
	}
}

func getGraphStatus(storeInfo map[string]string) [][2]string {
	graphStatus := make([][2]string, 0, len(storeInfo))
	for k, v := range storeInfo {
		graphStatus = append(graphStatus, [2]string{k, v})
	}
	return graphStatus
}

// getCgroupDriver returns the cgroup driver reported to Docker API clients.
//
// Rootless Podman using the cgroupfs manager cannot honor a cgroup parent
// chosen by the client: only the delegated subtree is writable, and container
// creation deliberately leaves the parent unset in that case.  Reporting
// "cgroupfs" leads clients to assume a rootful daemon and ask for a parent that
// cannot be created, so report "none" instead, matching what rootless Docker
// does.
func getCgroupDriver(cgroupManager string, isRootless bool) string {
	if isRootless && cgroupManager == config.CgroupfsCgroupsManager {
		return "none"
	}
	return cgroupManager
}

func getSecOpts(sysInfo *sysinfo.SysInfo, c *config.Config) []string {
	var secOpts []string
	if sysInfo.AppArmor {
		secOpts = append(secOpts, "name=apparmor")
	}
	if sysInfo.Seccomp {
		profile := "default"
		if c.Containers.SeccompProfile != "" && c.Containers.SeccompProfile != config.SeccompDefaultPath {
			profile = c.Containers.SeccompProfile
		}
		secOpts = append(secOpts, fmt.Sprintf("name=seccomp,profile=%s", profile))
	}
	if rootless.IsRootless() {
		secOpts = append(secOpts, "name=rootless")
	}
	if selinux.GetEnabled() {
		secOpts = append(secOpts, "name=selinux")
	}

	return secOpts
}

func getRuntimes(runtime *libpod.Runtime, configInfo *config.Config) map[string]dockerSystem.RuntimeWithStatus {
	runtimes := map[string]dockerSystem.RuntimeWithStatus{}
	for name, paths := range configInfo.Engine.OCIRuntimes {
		if len(paths) == 0 {
			continue
		}
		rt := dockerSystem.RuntimeWithStatus{}
		rt.Runtime = dockerSystem.Runtime{Path: paths[0], Args: nil}
		if features := runtime.RuntimeFeatures(name); features != "" {
			rt.Status = map[string]string{ociRuntimeFeaturesKey: features}
		}
		runtimes[name] = rt
	}
	return runtimes
}

func getFdCount() (count int) {
	count = -1
	if entries, err := os.ReadDir("/proc/self/fd"); err == nil {
		count = len(entries)
	}
	return count
}

// Just ignoring Container errors here...
func getContainersState(r *libpod.Runtime) map[define.ContainerStatus]int {
	states := map[define.ContainerStatus]int{}
	ctnrs, err := r.GetAllContainers()
	if err == nil {
		for _, ctnr := range ctnrs {
			state, err := ctnr.State()
			if err != nil {
				continue
			}
			states[state]++
		}
	}
	return states
}

func getEnv(value string) string {
	if v, exists := os.LookupEnv(strings.ToUpper(value)); exists {
		return v
	}
	if v, exists := os.LookupEnv(strings.ToLower(value)); exists {
		return v
	}
	return ""
}

func getDefaultAddressPools(configInfo *config.Config) []dockerSystem.NetworkAddressPool {
	// Convert DefaultSubnetPools to DefaultAddressPools
	if len(configInfo.Network.DefaultSubnetPools) == 0 {
		return nil
	}

	pools := make([]dockerSystem.NetworkAddressPool, 0, len(configInfo.Network.DefaultSubnetPools))
	for _, pool := range configInfo.Network.DefaultSubnetPools {
		if pool.Base == nil {
			continue
		}

		pfx, err := netip.ParsePrefix(pool.Base.String())
		if err != nil {
			continue
		}

		pools = append(pools, dockerSystem.NetworkAddressPool{
			Base: pfx,
			Size: pool.Size,
		})
	}

	return pools
}
