//go:build !remote

package generate

import (
	"context"
	"fmt"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/specgen"
)

// SpecGenToOCI returns the base configuration for the container.
func SpecGenToOCI(ctx context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, newImage *libimage.Image, mounts []spec.Mount, pod *libpod.Pod, finalCmd []string, compatibleOptions *libpod.InfraInherit) (*spec.Spec, error) {
	var imageOs string
	if newImage != nil {
		inspectData, err := newImage.Inspect(ctx, nil)
		if err != nil {
			return nil, err
		}
		imageOs = inspectData.Os
	} else {
		imageOs = "freebsd"
	}

	if imageOs != "freebsd" && imageOs != "linux" {
		return nil, fmt.Errorf("unsupported image OS: %s", imageOs)
	}

	g, err := generate.New(imageOs)
	if err != nil {
		return nil, err
	}

	g.SetProcessCwd(s.WorkDir)

	g.SetProcessArgs(finalCmd)

	if s.Terminal != nil {
		g.SetProcessTerminal(*s.Terminal)
	}

	for key, val := range s.Annotations {
		g.AddAnnotation(key, val)
	}

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

	// Linux emulatioon
	if imageOs == "linux" {
		var mounts []spec.Mount
		for _, m := range configSpec.Mounts {
			switch m.Destination {
			case "/proc":
				m.Type = "linprocfs"
				m.Options = []string{"nodev"}
				mounts = append(mounts, m)
				continue
			case "/sys":
				m.Type = "linsysfs"
				m.Options = []string{"nodev"}
				mounts = append(mounts, m)
				continue
			case "/dev", "/dev/pts", "/dev/shm", "/dev/mqueue":
				continue
			}
		}
		mounts = append(mounts,
			spec.Mount{
				Destination: "/dev",
				Type:        "devfs",
				Source:      "devfs",
				Options: []string{
					"ruleset=4",
					"rule=path shm unhide mode 1777",
				},
			},
			spec.Mount{
				Destination: "/dev/fd",
				Type:        "fdescfs",
				Source:      "fdesc",
				Options:     []string{},
			},
			spec.Mount{
				Destination: "/dev/shm",
				Type:        define.TypeTmpfs,
				Source:      "shm",
				Options:     []string{"notmpcopyup"},
			},
		)
		configSpec.Mounts = mounts
	}

	// Devices
	var userDevices []spec.LinuxDevice
	if !s.IsPrivileged() {
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

	return configSpec, nil
}

func WeightDevices(_ map[string]spec.LinuxWeightDevice) ([]spec.LinuxWeightDevice, error) {
	devs := []spec.LinuxWeightDevice{}
	return devs, nil
}
