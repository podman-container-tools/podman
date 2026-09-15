//go:build !remote

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/specgenutil"
	"github.com/docker/docker/api/types/mount"
	"go.podman.io/common/libimage"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/common/pkg/config"
	"go.podman.io/storage"
	"go.podman.io/storage/pkg/fileutils"
)

func CreateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		Name     string `schema:"name"`
		Platform string `schema:"platform"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// compatible configuration
	body := handlers.CreateContainerConfig{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("decode(): %w", err))
		return
	}

	// Override the container name in the body struct
	body.Name = query.Name

	if len(body.HostConfig.Links) > 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("bad parameter: %w", utils.ErrLinkNotSupport))
		return
	}
	rtc, err := runtime.GetConfig()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to get runtime config: %w", err))
		return
	}

	imageName, err := utils.NormalizeToDockerHub(r, body.Config.Image)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}
	body.Config.Image = imageName

	lookupImageOptions := libimage.LookupImageOptions{}
	if query.Platform != "" {
		var err error
		lookupImageOptions.OS, lookupImageOptions.Architecture, lookupImageOptions.Variant, err = parse.Platform(query.Platform)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("parsing platform: %w", err))
			return
		}
	}
	newImage, resolvedName, err := runtime.LibimageRuntime().LookupImage(body.Config.Image, &lookupImageOptions)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			utils.Error(w, http.StatusNotFound, fmt.Errorf("no such image: %w", err))
			return
		}

		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("looking up image: %w", err))
		return
	}

	// Take body structure and convert to cliopts
	cliOpts, args, err := cliOpts(body, rtc)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("make cli opts(): %w", err))
		return
	}

	imgNameOrID := newImage.ID()
	// if the img had multi names with the same sha256 ID, should use the InputName, not the ID
	if len(newImage.Names()) > 1 {
		if err := utils.IsRegistryReference(resolvedName); err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		// maybe the InputName has no tag, so use full name to display
		imgNameOrID = resolvedName
	}

	sg := specgen.NewSpecGenerator(imgNameOrID, cliOpts.RootFS)
	if err := specgenutil.FillOutSpecGen(sg, cliOpts, args); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("fill out specgen: %w", err))
		return
	}
	// moby always create the working directory
	localTrue := true
	sg.CreateWorkingDir = &localTrue
	// moby doesn't inherit /etc/hosts from host, but only overwrite if not set in containers.conf
	if rtc.Containers.BaseHostsFile == "" {
		sg.BaseHostsFile = "none"
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.ContainerCreate(r.Context(), sg)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("container create: %w", err))
		return
	}
	createResponse := entities.ContainerCreateResponse{
		ID:       report.Id,
		Warnings: []string{},
	}
	utils.WriteResponse(w, http.StatusCreated, createResponse)
}

func stringMaptoArray(m map[string]string) []string {
	a := make([]string, 0, len(m))
	for k, v := range m {
		a = append(a, fmt.Sprintf("%s=%s", k, v))
	}
	return a
}

// cliOpts converts a compat input struct to cliopts
func cliOpts(cc handlers.CreateContainerConfig, rtc *config.Config) (*entities.ContainerCreateOptions, []string, error) {
	var (
		capAdd     []string
		cappDrop   []string
		entrypoint *string
		init       bool
		specPorts  []types.PortMapping
	)

	if cc.HostConfig.Init != nil {
		init = *cc.HostConfig.Init
	}

	// Iterate devices and convert to CLI expected string
	devices := make([]string, 0, len(cc.HostConfig.Devices))
	for _, dev := range cc.HostConfig.Devices {
		devices = append(devices, fmt.Sprintf("%s:%s:%s", dev.PathOnHost, dev.PathInContainer, dev.CgroupPermissions))
	}
	for _, r := range cc.HostConfig.Resources.DeviceRequests {
		if r.Driver == "cdi" {
			devices = append(devices, r.DeviceIDs...)
		}
	}

	// iterate blkreaddevicebps
	readBps := make([]string, 0, len(cc.HostConfig.BlkioDeviceReadBps))
	for _, dev := range cc.HostConfig.BlkioDeviceReadBps {
		readBps = append(readBps, dev.String())
	}

	// iterate blkreaddeviceiops
	readIops := make([]string, 0, len(cc.HostConfig.BlkioDeviceReadIOps))
	for _, dev := range cc.HostConfig.BlkioDeviceReadIOps {
		readIops = append(readIops, dev.String())
	}

	// iterate blkwritedevicebps
	writeBps := make([]string, 0, len(cc.HostConfig.BlkioDeviceWriteBps))
	for _, dev := range cc.HostConfig.BlkioDeviceWriteBps {
		writeBps = append(writeBps, dev.String())
	}

	// iterate blkwritedeviceiops
	writeIops := make([]string, 0, len(cc.HostConfig.BlkioDeviceWriteIOps))
	for _, dev := range cc.HostConfig.BlkioDeviceWriteIOps {
		writeIops = append(writeIops, dev.String())
	}

	// entrypoint
	// can be a string or slice. if it is a slice, we need to
	// marshall it to json; otherwise it should just be the string
	// value
	if len(cc.Config.Entrypoint) > 0 {
		entrypoint = &cc.Config.Entrypoint[0]
		if len(cc.Config.Entrypoint) > 1 {
			b, err := json.Marshal(cc.Config.Entrypoint)
			if err != nil {
				return nil, nil, err
			}
			jsonString := string(b)
			entrypoint = &jsonString
		}
	} else if cc.Config.Entrypoint != nil {
		// Entrypoint in HTTP request is set, but it is an empty slice.
		// Set the entrypoint to empty string slice, because keeping it set to nil
		// would later fallback to default entrypoint.
		emptySlice := "[]"
		entrypoint = &emptySlice
	}

	// expose ports
	expose := make([]string, 0, len(cc.Config.ExposedPorts))
	for p := range cc.Config.ExposedPorts {
		expose = append(expose, fmt.Sprintf("%s/%s", p.Port(), p.Proto()))
	}

	// mounts type=tmpfs/bind,source=...,target=...=,opt=val
	volSources := make(map[string]bool)
	volDestinations := make(map[string]bool)
	mounts := make([]string, 0, len(cc.HostConfig.Mounts))
	var builder strings.Builder
	for _, m := range cc.HostConfig.Mounts {
		addField(&builder, "type", string(m.Type))
		addField(&builder, "source", m.Source)
		addField(&builder, "target", m.Target)

		// Store source/dest so we don't add duplicates if a volume is
		// also mentioned in cc.Volumes.
		// Which Docker Compose v2.0 does, for unclear reasons...
		volSources[m.Source] = true
		volDestinations[m.Target] = true

		if m.ReadOnly {
			addField(&builder, "ro", "true")
		}
		addField(&builder, "consistency", string(m.Consistency))
		// Map any specialized mount options that intersect between *Options and cli options
		switch m.Type {
		case mount.TypeBind:
			if m.BindOptions != nil {
				addField(&builder, "bind-propagation", string(m.BindOptions.Propagation))
				addField(&builder, "bind-nonrecursive", strconv.FormatBool(m.BindOptions.NonRecursive))
			}
		case mount.TypeTmpfs:
			if m.TmpfsOptions != nil {
				addField(&builder, "tmpfs-size", strconv.FormatInt(m.TmpfsOptions.SizeBytes, 10))
				addField(&builder, "tmpfs-mode", strconv.FormatUint(uint64(m.TmpfsOptions.Mode), 8))
			}
		case mount.TypeVolume:
			// All current VolumeOpts are handled above
			// See vendor/github.com/containers/common/pkg/parse/parse.go:ValidateVolumeOpts()
		}
		mounts = append(mounts, builder.String())
		builder.Reset()
	}

	// dns
	dns := make([]net.IP, 0, len(cc.HostConfig.DNS))
	for _, d := range cc.HostConfig.DNS {
		dns = append(dns, net.ParseIP(d))
	}

	// publish
	for port, pbs := range cc.HostConfig.PortBindings {
		for _, pb := range pbs {
			var hostport int
			var err error
			if pb.HostPort != "" {
				hostport, err = strconv.Atoi(pb.HostPort)
			}
			if err != nil {
				return nil, nil, err
			}
			tmpPort := types.PortMapping{
				HostIP:        pb.HostIP,
				ContainerPort: uint16(port.Int()),
				HostPort:      uint16(hostport),
				Range:         0,
				Protocol:      port.Proto(),
			}
			specPorts = append(specPorts, tmpPort)
		}
	}

	// special case for NetworkMode, the podman default is slirp4netns for
	// rootless but for better docker compat we want bridge. Do this only if
	// the default config in containers.conf wasn't overridden to use another
	// value than the default "private" one.
	netmode := string(cc.HostConfig.NetworkMode)
	configDefaultNetNS := rtc.Containers.NetNS
	if netmode == "" || netmode == "default" {
		if configDefaultNetNS == "" || configDefaultNetNS == string(specgen.Default) || configDefaultNetNS == string(specgen.Private) {
			netmode = "bridge"
		} else {
			netmode = configDefaultNetNS
		}
	}

	nsmode, networks, netOpts, err := specgen.ParseNetworkFlag([]string{netmode})
	if err != nil {
		return nil, nil, err
	}

	// network
	// Note: we cannot emulate compat exactly here. we only allow specifics of networks to be
	// defined when there is only one network.
	netInfo := entities.NetOptions{
		AddHosts:       cc.HostConfig.ExtraHosts,
		DNSOptions:     cc.HostConfig.DNSOptions,
		DNSSearch:      cc.HostConfig.DNSSearch,
		DNSServers:     dns,
		Network:        nsmode,
		PublishPorts:   specPorts,
		NetworkOptions: netOpts,
		NoHosts:        rtc.Containers.NoHosts,
	}

	// docker-compose sets the mac address on the container config instead
	// on the per network endpoint config
	//
	// This field is deprecated since API v1.44 where
	// EndpointSettings.MacAddress is used instead (and has precedence
	// below).  Let's still use it for backwards compat.
	containerMacAddress := cc.MacAddress //nolint:staticcheck

	// network names
	switch {
	case len(cc.NetworkingConfig.EndpointsConfig) > 0:
		endpointsConfig := cc.NetworkingConfig.EndpointsConfig
		networks := make(map[string]types.PerNetworkOptions, len(endpointsConfig))
		for netName, endpoint := range endpointsConfig {
			netOpts := types.PerNetworkOptions{}
			if endpoint != nil {
				netOpts.Aliases = endpoint.Aliases

				// if IP address is provided
				if len(endpoint.IPAddress) > 0 {
					staticIP := net.ParseIP(endpoint.IPAddress)
					if staticIP == nil {
						return nil, nil, fmt.Errorf("failed to parse the ip address %q", endpoint.IPAddress)
					}
					netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
				}

				if endpoint.IPAMConfig != nil {
					// if IPAMConfig.IPv4Address is provided
					if len(endpoint.IPAMConfig.IPv4Address) > 0 {
						staticIP := net.ParseIP(endpoint.IPAMConfig.IPv4Address)
						if staticIP == nil {
							return nil, nil, fmt.Errorf("failed to parse the ipv4 address %q", endpoint.IPAMConfig.IPv4Address)
						}
						netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
					}
					// if IPAMConfig.IPv6Address is provided
					if len(endpoint.IPAMConfig.IPv6Address) > 0 {
						staticIP := net.ParseIP(endpoint.IPAMConfig.IPv6Address)
						if staticIP == nil {
							return nil, nil, fmt.Errorf("failed to parse the ipv6 address %q", endpoint.IPAMConfig.IPv6Address)
						}
						netOpts.StaticIPs = append(netOpts.StaticIPs, staticIP)
					}
				}
				// If MAC address is provided
				if len(endpoint.MacAddress) > 0 {
					staticMac, err := net.ParseMAC(endpoint.MacAddress)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to parse the mac address %q", endpoint.MacAddress)
					}
					netOpts.StaticMAC = types.HardwareAddr(staticMac)
				} else if len(containerMacAddress) > 0 {
					// docker-compose only sets one mac address for the container on the container config
					// If there are more than one network attached it will end up on the first one,
					// which is not deterministic since we iterate a map. Not nice but this matches docker.
					staticMac, err := net.ParseMAC(containerMacAddress)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to parse the mac address %q", containerMacAddress)
					}
					netOpts.StaticMAC = types.HardwareAddr(staticMac)
					containerMacAddress = ""
				}
			}

			// Report configuration error in case bridge mode is not used.
			if !nsmode.IsBridge() && (len(netOpts.Aliases) > 0 || len(netOpts.StaticIPs) > 0 || len(netOpts.StaticMAC) > 0) {
				return nil, nil, fmt.Errorf("networks and static ip/mac address can only be used with Bridge mode networking")
			} else if nsmode.IsBridge() {
				// Docker CLI now always sends the end point config when using the default (bridge) mode
				// however podman configuration doesn't expect this to define this at all when not in bridge
				// mode and the podman server config might override the default network mode to something
				// else than bridge. So adapt to the podman expectation and define custom end point config
				// only when really using the bridge mode.
				networks[netName] = netOpts
			}
		}

		netInfo.Networks = networks
	case len(cc.HostConfig.NetworkMode) > 0:
		netInfo.Networks = networks
	}

	parsedTmp := make([]string, 0, len(cc.HostConfig.Tmpfs))
	for path, options := range cc.HostConfig.Tmpfs {
		finalString := path
		if options != "" {
			finalString += ":" + options
		}
		parsedTmp = append(parsedTmp, finalString)
	}

	// Note: several options here are marked as "don't need". this is based
	// on speculation by Matt and I. We think that these come into play later
	// like with start. We believe this is just a difference in podman/compat
	cliOpts := entities.ContainerCreateOptions{
		// Attach:            nil, // don't need?
		Authfile:     "",
		CapAdd:       append(capAdd, cc.HostConfig.CapAdd...),
		CapDrop:      append(cappDrop, cc.HostConfig.CapDrop...),
		CgroupParent: cc.HostConfig.CgroupParent,
		CIDFile:      cc.HostConfig.ContainerIDFile,
		CPUPeriod:    uint64(cc.HostConfig.CPUPeriod),
		CPUQuota:     cc.HostConfig.CPUQuota,
		CPURTPeriod:  uint64(cc.HostConfig.CPURealtimePeriod),
		CPURTRuntime: cc.HostConfig.CPURealtimeRuntime,
		CPUShares:    uint64(cc.HostConfig.CPUShares),
		// CPUS:              0, // don't need?
		CPUSetCPUs: cc.HostConfig.CpusetCpus,
		CPUSetMems: cc.HostConfig.CpusetMems,
		// Detach:            false, // don't need
		// DetachKeys:        "",    // don't need
		Devices:              devices,
		DeviceCgroupRule:     cc.HostConfig.DeviceCgroupRules,
		DeviceReadBPs:        readBps,
		DeviceReadIOPs:       readIops,
		DeviceWriteBPs:       writeBps,
		DeviceWriteIOPs:      writeIops,
		Entrypoint:           entrypoint,
		Env:                  cc.Config.Env,
		Expose:               expose,
		GroupAdd:             cc.HostConfig.GroupAdd,
		Hostname:             cc.Config.Hostname,
		ImageVolume:          "anonymous",
		Init:                 init,
		Interactive:          cc.Config.OpenStdin,
		IPC:                  string(cc.HostConfig.IpcMode),
		Label:                stringMaptoArray(cc.Config.Labels),
		LogDriver:            cc.HostConfig.LogConfig.Type,
		LogOptions:           stringMaptoArray(cc.HostConfig.LogConfig.Config),
		Name:                 cc.Name,
		OOMScoreAdj:          &cc.HostConfig.OomScoreAdj,
		Arch:                 "",
		OS:                   "",
		Variant:              "",
		PID:                  string(cc.HostConfig.PidMode),
		PIDsLimit:            cc.HostConfig.PidsLimit,
		Privileged:           cc.HostConfig.Privileged,
		PublishAll:           cc.HostConfig.PublishAllPorts,
		Quiet:                false,
		ReadOnly:             cc.HostConfig.ReadonlyRootfs,
		ReadWriteTmpFS:       true, // podman default
		Rm:                   cc.HostConfig.AutoRemove,
		Annotation:           stringMaptoArray(cc.HostConfig.Annotations),
		SecurityOpt:          cc.HostConfig.SecurityOpt,
		StopSignal:           cc.Config.StopSignal,
		StopTimeout:          rtc.Engine.StopTimeout, // podman default
		StorageOpts:          stringMaptoArray(cc.HostConfig.StorageOpt),
		Sysctl:               stringMaptoArray(cc.HostConfig.Sysctls),
		Systemd:              "true", // podman default
		TmpFS:                parsedTmp,
		TTY:                  cc.Config.Tty,
		EnvMerge:             cc.EnvMerge,
		UnsetEnv:             cc.UnsetEnv,
		UnsetEnvAll:          cc.UnsetEnvAll,
		User:                 cc.Config.User,
		UserNS:               string(cc.HostConfig.UsernsMode),
		UTS:                  string(cc.HostConfig.UTSMode),
		CgroupNS:             string(cc.HostConfig.CgroupnsMode),
		Mount:                mounts,
		VolumesFrom:          cc.HostConfig.VolumesFrom,
		Workdir:              cc.Config.WorkingDir,
		Net:                  &netInfo,
		HealthInterval:       define.DefaultHealthCheckInterval,
		HealthRetries:        define.DefaultHealthCheckRetries,
		HealthTimeout:        define.DefaultHealthCheckTimeout,
		HealthStartPeriod:    define.DefaultHealthCheckStartPeriod,
		HealthLogDestination: define.DefaultHealthCheckLocalDestination,
		HealthMaxLogCount:    define.DefaultHealthMaxLogCount,
		HealthMaxLogSize:     define.DefaultHealthMaxLogSize,
	}

	var ulimits []string
	if len(cc.HostConfig.Ulimits) > 0 {
		for _, ul := range cc.HostConfig.Ulimits {
			ulimits = append(ulimits, ul.String())
		}
		cliOpts.Ulimit = ulimits
	}

	if cc.HostConfig.Resources.NanoCPUs > 0 {
		if cliOpts.CPUPeriod != 0 || cliOpts.CPUQuota != 0 {
			return nil, nil, fmt.Errorf("NanoCpus conflicts with CpuPeriod and CpuQuota")
		}
		cliOpts.CPUPeriod = 100000
		cliOpts.CPUQuota = cc.HostConfig.Resources.NanoCPUs / 10000
	}

	// volumes
	for _, vol := range cc.HostConfig.Binds {
		cliOpts.Volume = append(cliOpts.Volume, vol)
		// Extract the destination so we don't add duplicate mounts in
		// the volumes phase.
		splitVol := specgen.SplitVolumeString(vol)
		switch len(splitVol) {
		case 1:
			volDestinations[vol] = true
		default:
			volSources[splitVol[0]] = true
			volDestinations[splitVol[1]] = true
		}
	}
	// Anonymous volumes are added differently from other volumes, in their
	// own special field, for reasons known only to Docker. Still use the
	// format of `-v` so we can just append them in there.
	// Unfortunately, these may be duplicates of existing mounts in Binds.
	// So... We need to catch that.
	// This also handles volumes duplicated between cc.HostConfig.Mounts and
	// cc.Volumes, as seen in compose v2.0.
	for vol := range cc.Volumes {
		if _, ok := volDestinations[vol]; ok {
			continue
		}
		cliOpts.Volume = append(cliOpts.Volume, vol)
	}
	// Make mount points for compat volumes
	for vol := range volSources {
		// This might be a named volume.
		// Assume it is if it's not an absolute path.
		if !filepath.IsAbs(vol) {
			continue
		}
		// If volume already exists, there is nothing to do
		if err := fileutils.Exists(vol); err == nil {
			continue
		}
		if err := os.MkdirAll(vol, 0o755); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return nil, nil, fmt.Errorf("making volume mountpoint for volume %s: %w", vol, err)
			}
		}
	}
	if len(cc.HostConfig.BlkioWeightDevice) > 0 {
		devices := make([]string, 0, len(cc.HostConfig.BlkioWeightDevice))
		for _, d := range cc.HostConfig.BlkioWeightDevice {
			devices = append(devices, d.String())
		}
		cliOpts.BlkIOWeightDevice = devices
	}
	if cc.HostConfig.BlkioWeight > 0 {
		cliOpts.BlkIOWeight = strconv.Itoa(int(cc.HostConfig.BlkioWeight))
	}

	if cc.HostConfig.Memory > 0 {
		cliOpts.Memory = strconv.Itoa(int(cc.HostConfig.Memory))
	}

	if cc.HostConfig.MemoryReservation > 0 {
		cliOpts.MemoryReservation = strconv.Itoa(int(cc.HostConfig.MemoryReservation))
	}

	cgroupsv2, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return nil, nil, err
	}
	if cc.HostConfig.MemorySwap > 0 && (!rootless.IsRootless() || (rootless.IsRootless() && cgroupsv2)) {
		cliOpts.MemorySwap = strconv.Itoa(int(cc.HostConfig.MemorySwap))
	}

	if cc.Config.StopTimeout != nil {
		cliOpts.StopTimeout = uint(*cc.Config.StopTimeout)
	}

	if cc.HostConfig.ShmSize > 0 {
		cliOpts.ShmSize = strconv.Itoa(int(cc.HostConfig.ShmSize))
	}

	if len(cc.HostConfig.RestartPolicy.Name) > 0 {
		policy := string(cc.HostConfig.RestartPolicy.Name)
		// only add restart count on failure
		if cc.HostConfig.RestartPolicy.IsOnFailure() {
			policy += fmt.Sprintf(":%d", cc.HostConfig.RestartPolicy.MaximumRetryCount)
		}
		cliOpts.Restart = policy
	}

	if cc.HostConfig.MemorySwappiness != nil && (!rootless.IsRootless() || rootless.IsRootless() && cgroupsv2 && rtc.Engine.CgroupManager == "systemd") {
		cliOpts.MemorySwappiness = *cc.HostConfig.MemorySwappiness
	} else {
		cliOpts.MemorySwappiness = -1
	}
	if cc.HostConfig.OomKillDisable != nil {
		cliOpts.OOMKillDisable = *cc.HostConfig.OomKillDisable
	}
	if cc.Config.Healthcheck != nil {
		// Encode healthcheck test as JSON to preserve arguments with spaces.
		// MakeHealthCheckFromCli will unmarshal this back to the original array.
		cmdJSON, err := json.Marshal(cc.Config.Healthcheck.Test)
		if err != nil {
			return nil, nil, err
		}
		cliOpts.HealthCmd = string(cmdJSON)
		if cc.Config.Healthcheck.Interval > 0 {
			cliOpts.HealthInterval = cc.Config.Healthcheck.Interval.String()
		}
		if cc.Config.Healthcheck.Retries > 0 {
			cliOpts.HealthRetries = uint(cc.Config.Healthcheck.Retries)
		}
		if cc.Config.Healthcheck.StartPeriod > 0 {
			cliOpts.HealthStartPeriod = cc.Config.Healthcheck.StartPeriod.String()
		}
		if cc.Config.Healthcheck.Timeout > 0 {
			cliOpts.HealthTimeout = cc.Config.Healthcheck.Timeout.String()
		}
	}

	// specgen assumes the image name is arg[0]
	cmd := make([]string, 0, 1+len(cc.Config.Cmd))
	cmd = append(cmd, cc.Config.Image)
	cmd = append(cmd, cc.Config.Cmd...)
	return &cliOpts, cmd, nil
}

// addField is a helper function to populate mount options
func addField(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}

	if b.Len() > 0 {
		b.WriteRune(',')
	}
	b.WriteString(name)
	b.WriteRune('=')
	b.WriteString(value)
}
