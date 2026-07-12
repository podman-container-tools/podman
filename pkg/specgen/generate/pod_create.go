//go:build !remote && (linux || freebsd)

package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutil"
)

func MakePod(p *entities.PodSpec, rt *libpod.Runtime) (_ *libpod.Pod, finalErr error) {
	var createdPod *libpod.Pod
	defer func() {
		if finalErr != nil && createdPod != nil {
			if _, err := rt.RemovePod(context.Background(), createdPod, true, true, nil); err != nil {
				logrus.Errorf("Removing pod: %v", err)
			}
		}
	}()
	if err := p.PodSpecGen.Validate(); err != nil {
		return nil, err
	}

	if p.PodSpecGen.ResourceLimits == nil {
		p.PodSpecGen.ResourceLimits = &specs.LinuxResources{}
	}

	if !p.PodSpecGen.NoInfra {
		imageName, err := PullInfraImage(rt, p.PodSpecGen.InfraImage)
		if err != nil {
			return nil, err
		}
		if len(imageName) > 0 {
			p.PodSpecGen.InfraImage = imageName
			p.PodSpecGen.InfraContainerSpec.RawImageName = imageName
		}
	}

	spec, err := MapSpec(&p.PodSpecGen)
	if err != nil {
		return nil, err
	}
	if err := specgen.FinishThrottleDevices(spec); err != nil {
		return nil, err
	}
	if err := specgen.WeightDevices(spec); err != nil {
		return nil, err
	}
	if spec.ResourceLimits != nil && spec.ResourceLimits.BlockIO != nil {
		p.PodSpecGen.ResourceLimits.BlockIO = spec.ResourceLimits.BlockIO
	}

	options, err := createPodOptions(&p.PodSpecGen)
	if err != nil {
		return nil, err
	}

	pod, err := rt.NewPod(context.Background(), p.PodSpecGen, options...)
	if err != nil {
		return nil, err
	}
	createdPod = pod

	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		if p.PodSpecGen.InfraContainerSpec.Name == "" {
			p.PodSpecGen.InfraContainerSpec.Name = pod.ID()[:12] + "-infra"
		}
		_, err = CompleteSpec(context.Background(), rt, p.PodSpecGen.InfraContainerSpec)
		if err != nil {
			return nil, err
		}
		p.PodSpecGen.InfraContainerSpec.User = "" // infraSpec user will get incorrectly assigned via the container creation process, overwrite here
		// infra's resource limits are used as a parsing tool,
		// we do not want infra to get these resources in its cgroup
		// make sure of that here.
		p.PodSpecGen.InfraContainerSpec.ResourceLimits = nil
		p.PodSpecGen.InfraContainerSpec.WeightDevice = nil

		rtSpec, spec, opts, err := MakeContainer(context.Background(), rt, p.PodSpecGen.InfraContainerSpec, false, nil)
		if err != nil {
			return nil, err
		}

		spec.Pod = pod.ID()
		opts = append(opts, rt.WithPod(pod))
		spec.CgroupParent = pod.CgroupParent()
		infraCtr, err := ExecuteCreate(context.Background(), rt, rtSpec, spec, true, opts...)
		if err != nil {
			return nil, err
		}
		pod, err = rt.AddInfra(context.Background(), pod, infraCtr)
		if err != nil {
			return nil, err
		}
	} else {
		// SavePod is used to save the pod state and trigger a create event even if infra is not created
		err := rt.SavePod(pod)
		if err != nil {
			return nil, err
		}
	}
	return pod, nil
}

func createPodOptions(p *specgen.PodSpecGenerator) ([]libpod.PodCreateOption, error) {
	var options []libpod.PodCreateOption

	if p.ShareParent == nil || (p.ShareParent != nil && *p.ShareParent) {
		options = append(options, libpod.WithPodParent())
	}
	if !p.NoInfra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := GetNamespaceOptions(p.SharedNamespaces, p.InfraContainerSpec.NetNS.IsHost())
		if err != nil {
			return nil, err
		}
		options = append(options, nsOptions...)
		// Use pod user and infra userns only when --userns is not set to host
		if !p.InfraContainerSpec.UserNS.IsHost() && !p.InfraContainerSpec.UserNS.IsDefault() {
			options = append(options, libpod.WithPodUser())
		}
	}

	if len(p.ServiceContainerID) > 0 {
		options = append(options, libpod.WithServiceContainer(p.ServiceContainerID))
	}

	if len(p.CgroupParent) > 0 {
		options = append(options, libpod.WithPodCgroupParent(p.CgroupParent))
	}
	if len(p.Labels) > 0 {
		options = append(options, libpod.WithPodLabels(p.Labels))
	}
	if len(p.Name) > 0 {
		options = append(options, libpod.WithPodName(p.Name))
	}
	if p.PodCreateCommand != nil {
		options = append(options, libpod.WithPodCreateCommand(p.PodCreateCommand))
	}

	if len(p.Hostname) > 0 {
		options = append(options, libpod.WithPodHostname(p.Hostname))
	}

	if p.ResourceLimits != nil {
		options = append(options, libpod.WithPodResources(*p.ResourceLimits))
	}

	options = append(options, libpod.WithPodExitPolicy(p.ExitPolicy))
	options = append(options, libpod.WithPodRestartPolicy(p.RestartPolicy))
	if p.RestartRetries != nil {
		options = append(options, libpod.WithPodRestartRetries(*p.RestartRetries))
	}

	return options, nil
}

// MapSpec modifies the already filled Infra specgenerator,
// replacing necessary values with those specified in pod creation
func MapSpec(p *specgen.PodSpecGenerator) (*specgen.SpecGenerator, error) {
	var spec *specgen.SpecGenerator
	if p.InfraContainerSpec != nil {
		spec = p.InfraContainerSpec
	} else {
		spec = &specgen.SpecGenerator{}
	}
	if len(p.PortMappings) > 0 {
		ports, err := ParsePortMapping(p.PortMappings, nil)
		if err != nil {
			return nil, err
		}
		spec.PortMappings = ports
	}
	switch p.NetNS.NSMode {
	case specgen.Default, "":
		if p.NoInfra {
			logrus.Debugf("No networking because the infra container is missing")
			break
		}
	case specgen.Bridge:
		spec.NetNS.NSMode = specgen.Bridge
		logrus.Debugf("Pod using bridge network mode")
	case specgen.Private:
		spec.NetNS.NSMode = specgen.Private
		logrus.Debugf("Pod will use default network mode")
	case specgen.Host:
		logrus.Debugf("Pod will use host networking")
		if len(spec.PortMappings) > 0 ||
			len(spec.Networks) > 0 ||
			spec.NetNS.NSMode == specgen.NoNetwork {
			return nil, fmt.Errorf("cannot set host network if network-related configuration is specified: %w", define.ErrInvalidArg)
		}
		spec.NetNS.NSMode = specgen.Host
	case specgen.Pasta:
		logrus.Debugf("Pod will use pasta")
		if spec.NetNS.NSMode != specgen.Host {
			spec.NetworkOptions = p.NetworkOptions
			spec.NetNS.NSMode = specgen.Pasta
		}
	case specgen.Path:
		logrus.Debugf("Pod will use namespace path networking")
		spec.NetNS.NSMode = specgen.Path
		spec.NetNS.Value = p.PodNetworkConfig.NetNS.Value
	case specgen.NoNetwork:
		logrus.Debugf("Pod will not use networking")
		if len(spec.PortMappings) > 0 ||
			len(spec.Networks) > 0 ||
			spec.NetNS.NSMode == specgen.Host {
			return nil, fmt.Errorf("cannot disable pod network if network-related configuration is specified: %w", define.ErrInvalidArg)
		}
		spec.NetNS.NSMode = specgen.NoNetwork
	default:
		return nil, fmt.Errorf("pods presently do not support network mode %s", p.NetNS.NSMode)
	}

	if len(p.InfraCommand) > 0 {
		spec.Entrypoint = p.InfraCommand
	}

	if len(p.HostAdd) > 0 {
		spec.HostAdd = p.HostAdd
	}
	if len(p.HostsFile) > 0 {
		spec.BaseHostsFile = p.HostsFile
	}
	if len(p.DNSServer) > 0 {
		spec.DNSServers = slices.Clone(p.DNSServer)
	}
	if len(p.DNSOption) > 0 {
		spec.DNSOptions = p.DNSOption
	}
	if len(p.DNSSearch) > 0 {
		spec.DNSSearch = p.DNSSearch
	}
	if p.NoManageResolvConf {
		localTrue := true
		spec.UseImageResolvConf = &localTrue
	}
	if len(p.Networks) > 0 {
		spec.Networks = p.Networks
	}
	if p.NoManageHosts {
		spec.UseImageHosts = &p.NoManageHosts
	}
	if p.NoManageHostname {
		spec.UseImageHostname = &p.NoManageHostname
	}

	if len(p.InfraConmonPidFile) > 0 {
		spec.ConmonPidFile = p.InfraConmonPidFile
	}

	if len(p.Sysctl) > 0 {
		spec.Sysctl = p.Sysctl
	}

	spec.Image = p.InfraImage
	return spec, nil
}

func PodConfigToSpec(rt *libpod.Runtime, spec *specgen.PodSpecGenerator, infraOptions *entities.ContainerCreateOptions, id string) (p *libpod.Pod, err error) {
	pod, err := rt.LookupPod(id)
	if err != nil {
		return nil, err
	}

	infraSpec := &specgen.SpecGenerator{}
	if pod.HasInfraContainer() {
		infraID, err := pod.InfraContainerID()
		if err != nil {
			return nil, err
		}
		_, _, err = ConfigToSpec(rt, infraSpec, infraID)
		if err != nil {
			return nil, err
		}

		infraSpec.Hostname = ""
		infraSpec.CgroupParent = ""
		infraSpec.Pod = "" // remove old pod...
		infraOptions.IsClone = true
		infraOptions.IsInfra = true

		n := infraSpec.Name
		_, err = rt.LookupContainer(n + "-clone")
		if err == nil { // if we found a ctr with this name, set it so the below switch can tell
			n += "-clone"
		}

		switch {
		case strings.Contains(n, "-clone"):
			ind := strings.Index(n, "-clone") + 6
			num, err := strconv.Atoi(n[ind:])
			if num == 0 && err != nil { // clone1 is hard to get with this logic, just check for it here.
				_, err = rt.LookupContainer(n + "1")
				if err != nil {
					infraSpec.Name = n + "1"
					break
				}
			} else {
				n = n[0:ind]
			}
			err = nil
			count := num
			for err == nil {
				count++
				tempN := n + strconv.Itoa(count)
				_, err = rt.LookupContainer(tempN)
			}
			n += strconv.Itoa(count)
			infraSpec.Name = n
		default:
			infraSpec.Name = n + "-clone"
		}

		err = specgenutil.FillOutSpecGen(infraSpec, infraOptions, []string{})
		if err != nil {
			return nil, err
		}

		out, err := CompleteSpec(context.Background(), rt, infraSpec)
		if err != nil {
			return nil, err
		}

		// Print warnings
		if len(out) > 0 {
			for _, w := range out {
				fmt.Println("Could not properly complete the spec as expected:")
				fmt.Fprintf(os.Stderr, "%s\n", w)
			}
		}

		spec.InfraContainerSpec = infraSpec
		matching, err := json.Marshal(infraSpec)
		if err != nil {
			return nil, err
		}

		// track name before unmarshal so we do not overwrite w/ infra
		name := spec.Name
		err = json.Unmarshal(matching, spec)
		if err != nil {
			return nil, err
		}

		if len(spec.InfraContainerSpec.Image) > 0 {
			spec.InfraImage = spec.InfraContainerSpec.Image
		}

		spec.Name = name
	}

	// need to reset hostname, name etc of both pod and infra
	spec.Hostname = ""

	return pod, nil
}
