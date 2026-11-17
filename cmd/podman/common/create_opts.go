package common

import (
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func ulimits() []string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Ulimits()
	}
	return nil
}

func cgroupConfig() string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Cgroups()
	}
	return ""
}

func devices() []string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Devices()
	}
	return nil
}

func Env() []string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Env()
	}
	return nil
}

func pidsLimit() int64 {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.PidsLimit()
	}
	return -1
}

func policy() string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Engine.PullPolicy
	}
	return ""
}

func shmSize() string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.ShmSize()
	}
	return ""
}

func volumes() []string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Volumes()
	}
	return nil
}

func LogDriver() string {
	if !registry.IsRemote() {
		return podmanConfig.ContainersConfDefaultsRO.Containers.LogDriver
	}
	return ""
}

// DefineCreateDefaults is used to initialize ctr create options before flag initialization
func DefineCreateDefaults(opts *entities.ContainerCreateOptions) {
	opts.LogDriver = LogDriver()
	opts.CgroupsMode = cgroupConfig()
	opts.MemorySwappiness = -1
	opts.ImageVolume = podmanConfig.ContainersConfDefaultsRO.Engine.ImageVolumeMode
	opts.Pull = policy()
	opts.ReadWriteTmpFS = true
	opts.SdNotifyMode = define.SdNotifyModeContainer
	opts.StopTimeout = podmanConfig.ContainersConfDefaultsRO.Engine.StopTimeout
	opts.Systemd = "true"
	opts.Timezone = podmanConfig.ContainersConfDefaultsRO.TZ()
	opts.Umask = podmanConfig.ContainersConfDefaultsRO.Umask()
	opts.Ulimit = ulimits()
	opts.SeccompPolicy = "default"
	opts.Volume = volumes()
	opts.HealthLogDestination = define.DefaultHealthCheckLocalDestination
	opts.HealthMaxLogCount = define.DefaultHealthMaxLogCount
	opts.HealthMaxLogSize = define.DefaultHealthMaxLogSize
}
