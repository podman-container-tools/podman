//go:build !remote && (linux || freebsd)

package generate

import (
	"fmt"
	"sort"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/namespaces"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/util"
)

const host = "host"

// userNSConflictsWithPod returns an error if the user namespace mode
// conflicts with pod namespace sharing requirements.
// Containers in a pod must use the same user namespace to avoid ownership and
// capability issues with shared resources.
func userNSConflictsWithPod(pod *libpod.Pod, mode specgen.NamespaceMode) error {
	if pod != nil && pod.HasInfraContainer() {
		// Allow modes that don't create a new user namespace
		switch mode {
		case specgen.FromPod, specgen.Default, specgen.Host, specgen.FromContainer:
			return nil
		default:
			return fmt.Errorf("cannot set user namespace mode when joining pod with infra container: %w", define.ErrInvalidArg)
		}
	}
	return nil
}

// Get the default namespace mode for any given namespace type.
func GetDefaultNamespaceMode(nsType string, cfg *config.Config, pod *libpod.Pod) (specgen.Namespace, error) {
	// The default for most is private
	toReturn := specgen.Namespace{}
	toReturn.NSMode = specgen.Private

	// Ensure case insensitivity
	nsType = strings.ToLower(nsType)

	// If the pod is not nil - check shared namespaces
	if pod != nil && pod.HasInfraContainer() {
		podMode := false
		switch {
		case nsType == "pid" && pod.SharesPID():
			if pod.NamespaceMode(spec.PIDNamespace) == host {
				toReturn.NSMode = specgen.Host
				return toReturn, nil
			}
			podMode = true
		case nsType == "ipc" && pod.SharesIPC():
			if pod.NamespaceMode(spec.IPCNamespace) == host {
				toReturn.NSMode = specgen.Host
				return toReturn, nil
			}
			podMode = true
		case nsType == "uts" && pod.SharesUTS():
			if pod.NamespaceMode(spec.UTSNamespace) == host {
				toReturn.NSMode = specgen.Host
				return toReturn, nil
			}
			podMode = true
		case nsType == "user" && pod.SharesUser():
			// user does not need a special check for host, this is already validated on pod creation
			// if --userns=host then pod.SharesUser == false
			podMode = true
		case nsType == "net" && pod.SharesNet():
			if pod.NetworkMode() == host {
				toReturn.NSMode = specgen.Host
				return toReturn, nil
			}
			podMode = true
		case nsType == "cgroup" && pod.SharesCgroup():
			if pod.NamespaceMode(spec.CgroupNamespace) == host {
				toReturn.NSMode = specgen.Host
				return toReturn, nil
			}
			podMode = true
		}
		if podMode {
			toReturn.NSMode = specgen.FromPod
			return toReturn, nil
		}
	}

	if cfg == nil {
		cfg = &config.Config{}
	}
	switch nsType {
	case "pid":
		return specgen.ParseNamespace(cfg.Containers.PidNS)
	case "ipc":
		return specgen.ParseIPCNamespace(cfg.Containers.IPCNS)
	case "uts":
		return specgen.ParseNamespace(cfg.Containers.UTSNS)
	case "user":
		return specgen.ParseUserNamespace(cfg.Containers.UserNS)
	case "cgroup":
		return specgen.ParseCgroupNamespace(cfg.Containers.CgroupNS)
	case "net":
		ns, _, _, _, err := specgen.ParseNetworkFlag(nil)
		return ns, err
	}

	return toReturn, fmt.Errorf("invalid namespace type %q passed: %w", nsType, define.ErrInvalidArg)
}

// namespaceOptions generates container creation options for all
// namespaces in a SpecGenerator.
// Pod is the pod the container will join. May be nil is the container is not
// joining a pod.
// TODO: Consider grouping options that are not directly attached to a namespace
// elsewhere.
func namespaceOptions(s *specgen.SpecGenerator, rt *libpod.Runtime, pod *libpod.Pod, imageData *libimage.ImageData) ([]libpod.CtrCreateOption, error) {
	toReturn := []libpod.CtrCreateOption{}

	// If pod is not nil, get infra container.
	var infraCtr *libpod.Container
	if pod != nil {
		infraID, err := pod.InfraContainerID()
		if err != nil {
			// This is likely to be of the fatal kind (pod was
			// removed) so hard fail
			return nil, fmt.Errorf("looking up pod %s infra container: %w", pod.ID(), err)
		}
		if infraID != "" {
			ctr, err := rt.GetContainer(infraID)
			if err != nil {
				return nil, fmt.Errorf("retrieving pod %s infra container %s: %w", pod.ID(), infraID, err)
			}
			infraCtr = ctr
		}
	}

	errNoInfra := fmt.Errorf("cannot use pod namespace as container is not joining a pod or pod has no infra container: %w", define.ErrInvalidArg)

	// PID
	switch s.PidNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithPIDNSFrom(infraCtr))
	case specgen.FromContainer:
		pidCtr, err := rt.LookupContainer(s.PidNS.Value)
		if err != nil {
			return nil, fmt.Errorf("looking up container to share pid namespace with: %w", err)
		}
		if rootless.IsRootless() && pidCtr.NamespaceMode(spec.PIDNamespace, pidCtr.ConfigNoCopy().Spec) == host {
			// Treat this the same as host, the problem is the runtime tries to do a
			// setns call and this will fail when it is the host ns as rootless user.
			s.PidNS.NSMode = specgen.Host
		} else {
			toReturn = append(toReturn, libpod.WithPIDNSFrom(pidCtr))
		}
	}

	// IPC
	switch s.IpcNS.NSMode {
	case specgen.Host:
		// Force use of host /dev/shm for host namespace
		toReturn = append(toReturn, libpod.WithShmDir("/dev/shm"))
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithIPCNSFrom(infraCtr))
		toReturn = append(toReturn, libpod.WithShmDir(infraCtr.ShmDir()))
	case specgen.FromContainer:
		ipcCtr, err := rt.LookupContainer(s.IpcNS.Value)
		if err != nil {
			return nil, fmt.Errorf("looking up container to share ipc namespace with: %w", err)
		}
		if ipcCtr.ConfigNoCopy().NoShmShare {
			return nil, fmt.Errorf("joining IPC of container %s is not allowed: non-shareable IPC (hint: use IpcMode:shareable for the donor container)", ipcCtr.ID())
		}
		if rootless.IsRootless() && ipcCtr.NamespaceMode(spec.IPCNamespace, ipcCtr.ConfigNoCopy().Spec) == host {
			// Treat this the same as host, the problem is the runtime tries to do a
			// setns call and this will fail when it is the host ns as rootless user.
			s.IpcNS.NSMode = specgen.Host
			toReturn = append(toReturn, libpod.WithShmDir("/dev/shm"))
		} else {
			toReturn = append(toReturn, libpod.WithIPCNSFrom(ipcCtr))
			if !ipcCtr.ConfigNoCopy().NoShm {
				toReturn = append(toReturn, libpod.WithShmDir(ipcCtr.ShmDir()))
			}
		}
	case specgen.None:
		toReturn = append(toReturn, libpod.WithNoShm(true))
	case specgen.Private:
		toReturn = append(toReturn, libpod.WithNoShmShare(true))
	}

	// UTS
	switch s.UtsNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		if pod.NamespaceMode(spec.UTSNamespace) == host {
			// adding infra as a nsCtr is not what we want to do when uts == host
			// this leads the new ctr to try to add an ns path which is should not in this mode
			logrus.Debug("pod has host uts, not adding infra as a nsCtr")
			s.UtsNS = specgen.Namespace{NSMode: specgen.Host}
		} else {
			toReturn = append(toReturn, libpod.WithUTSNSFrom(infraCtr))
		}
	case specgen.FromContainer:
		utsCtr, err := rt.LookupContainer(s.UtsNS.Value)
		if err != nil {
			return nil, fmt.Errorf("looking up container to share uts namespace with: %w", err)
		}
		if rootless.IsRootless() && utsCtr.NamespaceMode(spec.UTSNamespace, utsCtr.ConfigNoCopy().Spec) == host {
			// Treat this the same as host, the problem is the runtime tries to do a
			// setns call and this will fail when it is the host ns as rootless user.
			s.UtsNS.NSMode = specgen.Host
		} else {
			toReturn = append(toReturn, libpod.WithUTSNSFrom(utsCtr))
		}
	}

	// Validate that user namespace mode is compatible with pod.
	if err := userNSConflictsWithPod(pod, s.UserNS.NSMode); err != nil {
		return nil, err
	}

	switch s.UserNS.NSMode {
	case specgen.KeepID:
		opts, err := namespaces.UsernsMode(s.UserNS.String()).GetKeepIDOptions()
		if err != nil {
			return nil, err
		}
		if opts.UID == nil && opts.GID == nil {
			toReturn = append(toReturn, libpod.WithAddCurrentUserPasswdEntry())
		}

		// If user is not overridden, set user in the container
		// to user running Podman.
		if s.User == "" {
			_, uid, gid, err := util.GetKeepIDMapping(opts)
			if err != nil {
				return nil, err
			}
			toReturn = append(toReturn, libpod.WithUser(fmt.Sprintf("%d:%d", uid, gid)))
		}
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		// Inherit the user from the infra container if it is set and --user has not
		// been set explicitly
		if infraCtr.User() != "" && s.User == "" {
			toReturn = append(toReturn, libpod.WithUser(infraCtr.User()))
		}
		toReturn = append(toReturn, libpod.WithUserNSFrom(infraCtr))
	case specgen.FromContainer:
		userCtr, err := rt.LookupContainer(s.UserNS.Value)
		if err != nil {
			return nil, fmt.Errorf("looking up container to share user namespace with: %w", err)
		}
		toReturn = append(toReturn, libpod.WithUserNSFrom(userCtr))
	case specgen.Private:
	case specgen.Auto:
	case specgen.NoMap:
	case specgen.Path:
	}

	// This wipes the UserNS settings that get set from the infra container
	// when we are inheriting from the pod. So only apply this if the container
	// is not being created in a pod.
	if s.IDMappings != nil {
		if pod == nil {
			toReturn = append(toReturn, libpod.WithIDMappings(*s.IDMappings))
		}
	}
	if s.User != "" {
		toReturn = append(toReturn, libpod.WithUser(s.User))
	}
	if len(s.Groups) > 0 {
		toReturn = append(toReturn, libpod.WithGroups(s.Groups))
	}

	// Cgroup
	switch s.CgroupNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithCgroupNSFrom(infraCtr))
	case specgen.FromContainer:
		cgroupCtr, err := rt.LookupContainer(s.CgroupNS.Value)
		if err != nil {
			return nil, fmt.Errorf("looking up container to share cgroup namespace with: %w", err)
		}
		if rootless.IsRootless() && cgroupCtr.NamespaceMode(spec.CgroupNamespace, cgroupCtr.ConfigNoCopy().Spec) == host {
			// Treat this the same as host, the problem is the runtime tries to do a
			// setns call and this will fail when it is the host ns as rootless user.
			s.CgroupNS.NSMode = specgen.Host
		} else {
			toReturn = append(toReturn, libpod.WithCgroupNSFrom(cgroupCtr))
		}
	}

	if s.CgroupParent != "" {
		toReturn = append(toReturn, libpod.WithCgroupParent(s.CgroupParent))
	}

	if s.CgroupsMode != "" {
		toReturn = append(toReturn, libpod.WithCgroupsMode(s.CgroupsMode))
	}

	postConfigureNetNS := needPostConfigureNetNS(s)

	// Network
	portMappings, expose, err := createPortMappings(s, imageData)
	if err != nil {
		return nil, err
	}
	toReturn = append(toReturn, libpod.WithExposedPorts(expose))

	switch s.NetNS.NSMode {
	case specgen.FromPod:
		if pod == nil || infraCtr == nil {
			return nil, errNoInfra
		}
		toReturn = append(toReturn, libpod.WithNetNSFrom(infraCtr))
	case specgen.FromContainer:
		netCtr, err := rt.LookupContainer(s.NetNS.Value)
		if err != nil {
			return nil, fmt.Errorf("looking up container to share net namespace with: %w", err)
		}
		if rootless.IsRootless() && netCtr.NamespaceMode(spec.NetworkNamespace, netCtr.ConfigNoCopy().Spec) == host {
			// Treat this the same as host, the problem is the runtime tries to do a
			// setns call and this will fail when it is the host ns as rootless user.
			s.NetNS.NSMode = specgen.Host
		} else {
			toReturn = append(toReturn, libpod.WithNetNSFrom(netCtr))
		}
	case specgen.Pasta:
		val := "pasta"
		toReturn = append(toReturn, libpod.WithNetNS(portMappings, postConfigureNetNS, val, nil))
	case specgen.Bridge, specgen.Private, specgen.Default:
		rtConfig, err := rt.GetConfigNoCopy()
		if err != nil {
			return nil, err
		}
		// if no network was specified use add the default
		if len(s.Networks) == 0 {
			// no networks given but bridge is set so use default network
			s.Networks = map[string]types.PerNetworkOptions{
				rtConfig.Network.DefaultNetwork: {},
			}
		}
		// rename the "default" network to the correct default name
		if opts, ok := s.Networks["default"]; ok {
			s.Networks[rtConfig.Network.DefaultNetwork] = opts
			delete(s.Networks, "default")

			if s.NetworkOrder != nil {
				newNetworkOrder := make([]string, 0, len(s.NetworkOrder))
				for _, net := range s.NetworkOrder {
					if net == "default" {
						newNetworkOrder = append(newNetworkOrder, rtConfig.Network.DefaultNetwork)
					} else {
						newNetworkOrder = append(newNetworkOrder, net)
					}
				}
				s.NetworkOrder = newNetworkOrder
			}
		}

		finalNetworks := make([]types.NamedPerNetworkOptions, 0, len(s.Networks))
		if s.NetworkOrder != nil {
			if len(s.NetworkOrder) != len(s.Networks) {
				return nil, fmt.Errorf("the NetworkOrder struct must have the same length as Networks: %d vs %d", len(s.NetworkOrder), len(s.Networks))
			}
			for _, n := range s.NetworkOrder {
				network, ok := s.Networks[n]
				if !ok {
					return nil, fmt.Errorf("key found in NetworkOrder that is not in the Networks map: %s", n)
				}
				finalNetworks = append(finalNetworks, types.NamedPerNetworkOptions{
					Name:              n,
					PerNetworkOptions: network,
				})
			}
		} else {
			keys := make([]string, 0, len(s.Networks))
			for key := range s.Networks {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				finalNetworks = append(finalNetworks, types.NamedPerNetworkOptions{
					Name:              key,
					PerNetworkOptions: s.Networks[key],
				})
			}
		}

		toReturn = append(toReturn, libpod.WithNetNS(portMappings, postConfigureNetNS, "bridge", finalNetworks))
	}

	if s.UseImageHosts != nil && *s.UseImageHosts {
		toReturn = append(toReturn, libpod.WithUseImageHosts())
	} else if len(s.HostAdd) > 0 {
		toReturn = append(toReturn, libpod.WithHosts(s.HostAdd))
	}
	if s.UseImageHostname != nil && *s.UseImageHostname {
		toReturn = append(toReturn, libpod.WithUseImageHostname())
	}
	if len(s.DNSSearch) > 0 {
		toReturn = append(toReturn, libpod.WithDNSSearch(s.DNSSearch))
	}
	if s.UseImageResolvConf != nil && *s.UseImageResolvConf {
		toReturn = append(toReturn, libpod.WithUseImageResolvConf())
	} else if len(s.DNSServers) > 0 {
		var dnsServers []string
		for _, d := range s.DNSServers {
			dnsServers = append(dnsServers, d.String())
		}
		toReturn = append(toReturn, libpod.WithDNS(dnsServers))
	}
	if len(s.DNSOptions) > 0 {
		toReturn = append(toReturn, libpod.WithDNSOption(s.DNSOptions))
	}
	if s.NetworkOptions != nil {
		toReturn = append(toReturn, libpod.WithNetworkOptions(s.NetworkOptions))
	}

	return toReturn, nil
}

// GetNamespaceOptions transforms a slice of kernel namespaces
// into a slice of pod create options. Currently, not all
// kernel namespaces are supported, and they will be returned in an error
func GetNamespaceOptions(ns []string, _ bool) ([]libpod.PodCreateOption, error) {
	var options []libpod.PodCreateOption
	var erroredOptions []libpod.PodCreateOption
	if ns == nil {
		// set the default namespaces
		ns = strings.Split(specgen.DefaultKernelNamespaces, ",")
	}
	for _, toShare := range ns {
		switch toShare {
		case "cgroup":
			options = append(options, libpod.WithPodCgroup())
		case "net":
			options = append(options, libpod.WithPodNet())
		case "mnt":
			return erroredOptions, fmt.Errorf("mount sharing functionality not supported on pod level")
		case "pid":
			options = append(options, libpod.WithPodPID())
		case "user":
			continue
		case "ipc":
			options = append(options, libpod.WithPodIPC())
		case "uts":
			options = append(options, libpod.WithPodUTS())
		case "":
		case "none":
			return erroredOptions, nil
		default:
			return erroredOptions, fmt.Errorf("invalid kernel namespace to share: %s. Options are: cgroup, ipc, net, pid, uts or none", toShare)
		}
	}
	return options, nil
}
