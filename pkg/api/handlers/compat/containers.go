//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/storage"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/filters"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/ps"
	"go.podman.io/podman/v6/pkg/signal"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/util"
)

func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	query := struct {
		Force         bool  `schema:"force"`
		Ignore        bool  `schema:"ignore"`
		Depend        bool  `schema:"depend"`
		Link          bool  `schema:"link"`
		Timeout       *uint `schema:"timeout"`
		DockerVolumes bool  `schema:"v"`
		LibpodVolumes bool  `schema:"volumes"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.RmOptions{
		Force:  query.Force,
		Ignore: query.Ignore,
	}
	if utils.IsLibpodRequest(r) {
		options.Volumes = query.LibpodVolumes
		options.Timeout = query.Timeout
		options.Depend = query.Depend
	} else {
		if query.Link {
			utils.Error(w, http.StatusBadRequest, utils.ErrLinkNotSupport)
			return
		}
		options.Volumes = query.DockerVolumes
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	reports, err := containerEngine.ContainerRm(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}
	if len(reports) > 0 && reports[0].Err != nil {
		err = reports[0].Err
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, reports)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		All   bool `schema:"all"`
		Limit int  `schema:"limit"`
		Size  bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to decode filter parameters for %s: %w", r.URL.String(), err))
		return
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	filterFuncs := make([]libpod.ContainerFilter, 0, len(*filterMap))
	all := query.All || query.Limit > 0
	if len(*filterMap) > 0 {
		for k, v := range *filterMap {
			generatedFunc, err := filters.GenerateContainerFilterFuncs(k, v, runtime)
			if err != nil {
				utils.InternalServerError(w, err)
				return
			}
			filterFuncs = append(filterFuncs, generatedFunc)
		}
	}

	// Docker thinks that if status is given as an input, then we should override
	// the all setting and always deal with all containers.
	if len((*filterMap)["status"]) > 0 {
		all = true
	}
	if !all {
		runningOnly, err := filters.GenerateContainerFilterFuncs("status", []string{define.ContainerStateRunning.String()}, runtime)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		filterFuncs = append(filterFuncs, runningOnly)
	}

	containers, err := runtime.GetContainers(false, filterFuncs...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if _, found := r.URL.Query()["limit"]; found && query.Limit > 0 {
		// Sort the libpod containers
		sort.Sort(ps.SortCreateTime{SortContainers: containers})
		// we should perform the lopping before we start getting
		// the expensive information on containers
		if len(containers) > query.Limit {
			containers = containers[:query.Limit]
		}
	}
	list := make([]*handlers.Container, 0, len(containers))
	// only include health if API version is >= 1.52 or no version is given (latest)
	includeHealth := false
	if _, err := utils.SupportedVersion(r, ">= 1.52.0"); err == nil || errors.Is(err, apiutil.ErrVersionNotGiven) {
		includeHealth = true
	}
	for _, ctnr := range containers {
		api, err := LibpodToContainer(ctnr, query.Size, includeHealth)
		if err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) {
				// container was removed between the initial fetch of the list and conversion
				logrus.Debugf("Container %s removed between initial fetch and conversion, ignoring in output", ctnr.ID())
				continue
			}
			utils.InternalServerError(w, err)
			return
		}
		list = append(list, api)
	}
	utils.WriteResponse(w, http.StatusOK, list)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	name := utils.GetName(r)
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	api, err := LibpodToContainerJSON(ctnr, query.Size)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if _, err := utils.SupportedVersion(r, ">=1.52.0"); err == nil || errors.Is(err, apiutil.ErrVersionNotGiven) {
		if api.NetworkSettings != nil {
			api.NetworkSettings.SecondaryIPAddresses = nil
			api.NetworkSettings.SecondaryIPv6Addresses = nil
		}
	}
	utils.WriteResponse(w, http.StatusOK, api)
}

func KillContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/kill
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		Signal string `schema:"signal"`
	}{
		Signal: "KILL",
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	name := utils.GetName(r)
	options := entities.KillOptions{
		Signal: query.Signal,
	}
	report, err := containerEngine.ContainerKill(r.Context(), []string{name}, options)
	if err != nil {
		if errors.Is(err, define.ErrCtrStateInvalid) ||
			errors.Is(err, define.ErrCtrStopped) {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}

		utils.InternalServerError(w, err)
		return
	}

	if len(report) > 0 && report[0].Err != nil {
		if errors.Is(report[0].Err, define.ErrCtrStateInvalid) ||
			errors.Is(report[0].Err, define.ErrCtrStopped) {
			utils.Error(w, http.StatusConflict, report[0].Err)
			return
		}
		utils.InternalServerError(w, report[0].Err)
		return
	}
	// Docker waits for the container to stop if the signal is 0 or
	// SIGKILL.
	if !utils.IsLibpodRequest(r) {
		sig, err := signal.ParseSignalNameOrNumber(query.Signal)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		if sig == 0 || sig == syscall.SIGKILL {
			opts := entities.WaitOptions{
				Conditions: []string{define.ContainerStateExited.String(), define.ContainerStateStopped.String()},
				Interval:   time.Millisecond * 250,
			}
			if _, err := containerEngine.ContainerWait(r.Context(), []string{name}, opts); err != nil {
				utils.Error(w, http.StatusInternalServerError, err)
				return
			}
		}
	}
	// Success
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	// /{version}/containers/(name)/wait
	utils.WaitContainerDocker(w, r)
}

//nolint:staticcheck
func convertSecondaryIPPrefixLen(input *define.InspectNetworkSettings, output *handlers.LegacyNetworkSettings) {
	for index, ip := range input.SecondaryIPAddresses {
		output.SecondaryIPAddresses[index].PrefixLen = ip.PrefixLength
	}
	for index, ip := range input.SecondaryIPv6Addresses {
		output.SecondaryIPv6Addresses[index].PrefixLen = ip.PrefixLength
	}
}

func LibpodToContainer(l *libpod.Container, sz bool, includeHealth bool) (*handlers.Container, error) {
	imageID, imageName := l.Image()

	var (
		err        error
		sizeRootFs int64
		sizeRW     int64
		state      define.ContainerStatus
		status     string
		hostConfig = struct {
			NetworkMode string            `json:",omitempty"`
			Annotations map[string]string `json:",omitempty"`
		}{}
	)

	if state, err = l.State(); err != nil {
		return nil, err
	}
	stateStr := state.String()

	// Some docker states are not the same as ours. This makes sure the state string stays true to the Docker API.
	// "stopped" and "stopping" are podman-internal states with no Docker equivalent;
	// map them to the nearest Docker state so the compat API never leaks internal states.
	switch state {
	case define.ContainerStateCreated:
		stateStr = define.ContainerStateConfigured.String()
	case define.ContainerStateStopped:
		stateStr = define.ContainerStateExited.String()
	case define.ContainerStateStopping:
		stateStr = define.ContainerStateRunning.String()
	}

	switch state {
	case define.ContainerStateConfigured, define.ContainerStateCreated:
		status = "Created"
	case define.ContainerStateStopped, define.ContainerStateExited:
		exitCode, _, err := l.ExitCode()
		if err != nil {
			return nil, err
		}
		finishedTime, err := l.FinishedTime()
		if err != nil {
			return nil, err
		}
		status = fmt.Sprintf("Exited (%d) %s ago", exitCode, units.HumanDuration(time.Since(finishedTime)))
	case define.ContainerStateRunning, define.ContainerStatePaused:
		startedTime, err := l.StartedTime()
		if err != nil {
			return nil, err
		}
		status = fmt.Sprintf("Up %s", units.HumanDuration(time.Since(startedTime)))
		if state == define.ContainerStatePaused {
			status += " (Paused)"
		}
	case define.ContainerStateRemoving:
		status = "Removal In Progress"
	case define.ContainerStateStopping:
		status = "Stopping"
	default:
		status = "Unknown"
	}

	if sz {
		if sizeRW, err = l.RWSize(); err != nil {
			return nil, err
		}
		if sizeRootFs, err = l.RootFsSize(); err != nil {
			return nil, err
		}
	}

	inspect, err := l.Inspect(false)
	if err != nil {
		return nil, err
	}

	ports := []container.PortSummary{}
	for portKey, bindings := range inspect.NetworkSettings.Ports {
		portNum, proto, ok := strings.Cut(portKey, "/")
		if !ok {
			return nil, fmt.Errorf("PORT/PROTOCOL format required for %q", portKey)
		}

		containerPort, err := strconv.Atoi(portNum)
		if err != nil {
			return nil, err
		}

		if len(bindings) == 0 {
			// Exposed but not published
			ports = append(ports, container.PortSummary{
				PrivatePort: uint16(containerPort),
				Type:        proto,
			})
		} else {
			for _, b := range bindings {
				hostPortInt, err := strconv.Atoi(b.HostPort)
				if err != nil {
					return nil, fmt.Errorf("invalid HostPort: %v", err)
				}
				addr := netip.Addr{}
				if b.HostIP != "" {
					addr, err = netip.ParseAddr(b.HostIP)
					if err != nil {
						return nil, fmt.Errorf("invalid HostIP: %v", err)
					}
				}

				ports = append(ports, container.PortSummary{
					IP:          addr,
					PrivatePort: uint16(containerPort),
					PublicPort:  uint16(hostPortInt),
					Type:        proto,
				})
			}
		}
	}

	n, err := json.Marshal(inspect.NetworkSettings)
	if err != nil {
		return nil, err
	}
	networkSettings := container.NetworkSettingsSummary{}
	if err := json.Unmarshal(n, &networkSettings); err != nil {
		return nil, err
	}

	m, err := json.Marshal(inspect.Mounts)
	if err != nil {
		return nil, err
	}
	mounts := []container.MountPoint{}
	if err := json.Unmarshal(m, &mounts); err != nil {
		return nil, err
	}

	var healthSummary *container.HealthSummary
	if includeHealth {
		healthSummary = &container.HealthSummary{
			Status: container.NoHealthcheck,
		}
		if l.HasHealthCheck() {
			switch state {
			case define.ContainerStateRunning, define.ContainerStatePaused,
				define.ContainerStateConfigured, define.ContainerStateCreated,
				define.ContainerStateStopping:
				if inspect.State.Health != nil && inspect.State.Health.Status != "" {
					healthSummary.Status = container.HealthStatus(inspect.State.Health.Status)
					healthSummary.FailingStreak = inspect.State.Health.FailingStreak
				} else {
					healthSummary.Status = container.Starting
				}
			default:
				healthSummary.Status = container.NoHealthcheck
			}
		}
	}

	hc := inspect.HostConfig
	if hc != nil {
		hostConfig.NetworkMode = inspect.HostConfig.NetworkMode
		hostConfig.Annotations = inspect.HostConfig.Annotations
	}

	return &handlers.Container{
		Summary: container.Summary{
			ID:              l.ID(),
			Names:           []string{fmt.Sprintf("/%s", l.Name())},
			Image:           imageName,
			ImageID:         "sha256:" + imageID,
			Command:         strings.Join(l.Command(), " "),
			Created:         l.CreatedTime().Unix(),
			Ports:           ports,
			SizeRw:          sizeRW,
			SizeRootFs:      sizeRootFs,
			Labels:          l.Labels(),
			State:           container.ContainerState(stateStr),
			Status:          status,
			Health:          healthSummary,
			HostConfig:      hostConfig,
			NetworkSettings: &networkSettings,
			Mounts:          mounts,
		},
	}, nil
}

//nolint:staticcheck
func LibpodToContainerJSON(l *libpod.Container, sz bool) (*handlers.LegacyImageInspect, error) {
	imageID, imageName := l.Image()
	inspect, err := l.Inspect(sz)
	if err != nil {
		return nil, err
	}
	// Docker uses UTC
	if inspect != nil && inspect.State != nil {
		inspect.State.StartedAt = inspect.State.StartedAt.UTC()
		inspect.State.FinishedAt = inspect.State.FinishedAt.UTC()
	}
	i, err := json.Marshal(inspect.State)
	if err != nil {
		return nil, err
	}
	state := container.State{}
	if err := json.Unmarshal(i, &state); err != nil {
		return nil, err
	}

	// docker considers paused to be running
	if state.Paused {
		state.Running = true
	}

	// map our statuses to Docker's statuses
	switch string(state.Status) {
	case define.ContainerStateConfigured.String(), define.ContainerStateCreated.String():
		state.Status = "created"
	case define.ContainerStateRunning.String(), define.ContainerStateStopping.String():
		state.Status = "running"
	case define.ContainerStatePaused.String():
		state.Status = "paused"
	case define.ContainerStateRemoving.String():
		state.Status = "removing"
	case define.ContainerStateStopped.String(), define.ContainerStateExited.String():
		state.Status = "exited"
	default:
		state.Status = "" // unknown state
	}

	if l.HasHealthCheck() {
		state.Health = &container.Health{}
		if inspect.State.Health != nil {
			if inspect.State.Health.Status != "" {
				state.Health.Status = container.HealthStatus(inspect.State.Health.Status)
			} else if state.Status == "running" || state.Status == "created" {
				state.Health.Status = container.Starting
			}
			state.Health.FailingStreak = inspect.State.Health.FailingStreak
			log := inspect.State.Health.Log

			for _, item := range log {
				res := &container.HealthcheckResult{}
				s, err := time.Parse(time.RFC3339Nano, item.Start)
				if err != nil {
					return nil, err
				}
				e, err := time.Parse(time.RFC3339Nano, item.End)
				if err != nil {
					return nil, err
				}
				res.Start = s
				res.End = e
				res.ExitCode = item.ExitCode
				res.Output = item.Output
				state.Health.Log = append(state.Health.Log, res)
			}
		}
	}

	formatCapabilities(inspect.HostConfig.CapDrop)
	formatCapabilities(inspect.HostConfig.CapAdd)

	h, err := json.Marshal(inspect.HostConfig)
	if err != nil {
		return nil, err
	}
	hc := container.HostConfig{}
	if err := json.Unmarshal(h, &hc); err != nil {
		return nil, err
	}
	sort.Strings(hc.Binds)

	// Map CgroupMode to CgroupnsMode for Docker API compatibility
	switch inspect.HostConfig.CgroupMode {
	case "private":
		hc.CgroupnsMode = container.CgroupnsModePrivate
	case "host":
		hc.CgroupnsMode = container.CgroupnsModeHost
	}

	// k8s-file == json-file
	if hc.LogConfig.Type == define.KubernetesLogging {
		hc.LogConfig.Type = define.JSONLogging
	}

	graphDriver := storage.DriverData{
		Name: inspect.GraphDriver.Name,
		Data: inspect.GraphDriver.Data,
	}

	cb := handlers.LegacyImageInspect{ //nolint:staticcheck
		InspectResponse: container.InspectResponse{
			ID:              l.ID(),
			Created:         l.CreatedTime().UTC().Format(time.RFC3339Nano), // Docker uses UTC
			Path:            inspect.Path,
			Args:            inspect.Args,
			State:           &state,
			Image:           "sha256:" + imageID,
			ResolvConfPath:  inspect.ResolvConfPath,
			HostnamePath:    inspect.HostnamePath,
			HostsPath:       inspect.HostsPath,
			LogPath:         l.LogPath(),
			Name:            fmt.Sprintf("/%s", l.Name()),
			RestartCount:    int(inspect.RestartCount),
			Driver:          inspect.Driver,
			Platform:        "linux",
			MountLabel:      inspect.MountLabel,
			ProcessLabel:    inspect.ProcessLabel,
			AppArmorProfile: inspect.AppArmorProfile,
			ExecIDs:         inspect.ExecIDs,
			HostConfig:      &hc,
			GraphDriver:     &graphDriver,
			SizeRw:          inspect.SizeRw,
			SizeRootFs:      &inspect.SizeRootFs,
		},
	}

	// set Path and Args
	processArgs := l.Config().Spec.Process.Args
	if len(processArgs) > 0 {
		cb.Path = processArgs[0]
	}
	if len(processArgs) > 1 {
		cb.Args = processArgs[1:]
	}
	stopTimeout := int(l.StopTimeout())

	var healthcheck *container.HealthConfig
	if inspect.Config.Healthcheck != nil {
		healthcheck = &container.HealthConfig{
			Test:        inspect.Config.Healthcheck.Test,
			Interval:    inspect.Config.Healthcheck.Interval,
			Timeout:     inspect.Config.Healthcheck.Timeout,
			StartPeriod: inspect.Config.Healthcheck.StartPeriod,
			Retries:     inspect.Config.Healthcheck.Retries,
		}
	}

	// Convert to moby PortSet.
	var exposedPorts network.PortSet
	if len(inspect.Config.ExposedPorts) > 0 {
		exposedPorts = make(network.PortSet, len(inspect.Config.ExposedPorts))
		for p := range inspect.Config.ExposedPorts {
			mp, err := network.ParsePort(p)
			if err != nil {
				return nil, fmt.Errorf("invalid exposed port %q: %w", p, err)
			}
			exposedPorts[mp] = struct{}{}
		}
	}

	config := handlers.ContainerConfig{
		Config: container.Config{
			Hostname:        l.Hostname(),
			Domainname:      inspect.Config.DomainName,
			User:            l.User(),
			AttachStdin:     inspect.Config.AttachStdin,
			AttachStdout:    inspect.Config.AttachStdout,
			AttachStderr:    inspect.Config.AttachStderr,
			ExposedPorts:    exposedPorts,
			Tty:             inspect.Config.Tty,
			OpenStdin:       inspect.Config.OpenStdin,
			StdinOnce:       inspect.Config.StdinOnce,
			Env:             inspect.Config.Env,
			Cmd:             l.Command(),
			Healthcheck:     healthcheck,
			ArgsEscaped:     false,
			Image:           imageName,
			Volumes:         nil,
			WorkingDir:      l.WorkingDir(),
			Entrypoint:      l.Entrypoint(),
			NetworkDisabled: false,
			OnBuild:         nil,
			Labels:          l.Labels(),
			StopSignal:      strconv.Itoa(int(l.StopSignal())),
			StopTimeout:     &stopTimeout,
			Shell:           nil,
		},
		MacAddress: "",
	}

	m, err := json.Marshal(inspect.Mounts)
	if err != nil {
		return nil, err
	}
	mounts := []container.MountPoint{}
	if err := json.Unmarshal(m, &mounts); err != nil {
		return nil, err
	}

	p, err := json.Marshal(inspect.NetworkSettings.Ports)
	if err != nil {
		return nil, err
	}
	ports := nat.PortMap{}
	if err := json.Unmarshal(p, &ports); err != nil {
		return nil, err
	}

	n, err := json.Marshal(inspect.NetworkSettings)
	if err != nil {
		return nil, err
	}

	networkSettings := handlers.LegacyNetworkSettings{} //nolint:staticcheck
	if err := json.Unmarshal(n, &networkSettings); err != nil {
		return nil, err
	}

	convertSecondaryIPPrefixLen(inspect.NetworkSettings, &networkSettings)

	// do not report null instead use an empty map
	if networkSettings.Networks == nil {
		networkSettings.Networks = map[string]*network.EndpointSettings{}
	}

	cb.Mounts = mounts
	cb.Config = &config
	cb.NetworkSettings = &networkSettings
	return &cb, nil
}

func formatCapabilities(slice []string) {
	for i := range slice {
		slice[i] = strings.TrimPrefix(slice[i], "CAP_")
	}
}

func RenameContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)

	name := utils.GetName(r)
	query := struct {
		Name string `schema:"name"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	if _, err := runtime.RenameContainer(r.Context(), ctr, query.Name); err != nil {
		if errors.Is(err, define.ErrPodExists) || errors.Is(err, define.ErrCtrExists) {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func UpdateContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	options := new(container.UpdateConfig)
	if err := utils.ReadJSONFromBody(r, &options); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	// Only handle the bits of update that Docker uses as examples.
	// For example, the update API claims to be able to update devices for
	// existing containers... Which I am very dubious about.
	// Ignore bits like that unless someone asks us for them.

	// We're going to be editing this, so we have to deep-copy to not affect
	// the container's own resources
	resources := new(spec.LinuxResources)
	oldResources := ctr.LinuxResources()
	if oldResources != nil {
		if err := libpod.JSONDeepCopy(oldResources, resources); err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("copying old resource limits: %w", err))
			return
		}
	}

	// CPU limits
	cpu := resources.CPU
	if cpu == nil {
		cpu = new(spec.LinuxCPU)
	}
	useCPU := false
	if options.CPUShares != 0 {
		shares := uint64(options.CPUShares)
		cpu.Shares = &shares
		useCPU = true
	}
	if options.CPUPeriod != 0 {
		period := uint64(options.CPUPeriod)
		cpu.Period = &period
		useCPU = true
	}
	if options.CPUQuota != 0 {
		cpu.Quota = &options.CPUQuota
		useCPU = true
	}
	if options.CPURealtimeRuntime != 0 {
		cpu.RealtimeRuntime = &options.CPURealtimeRuntime
		useCPU = true
	}
	if options.CPURealtimePeriod != 0 {
		period := uint64(options.CPURealtimePeriod)
		cpu.RealtimePeriod = &period
		useCPU = true
	}
	if options.CpusetCpus != "" {
		cpu.Cpus = options.CpusetCpus
		useCPU = true
	}
	if options.CpusetMems != "" {
		cpu.Mems = options.CpusetMems
		useCPU = true
	}
	if useCPU {
		resources.CPU = cpu
	}

	// Memory limits
	mem := resources.Memory
	if mem == nil {
		mem = new(spec.LinuxMemory)
	}
	useMem := false
	if options.Memory != 0 {
		mem.Limit = &options.Memory
		useMem = true
	}
	if options.MemorySwap != 0 {
		mem.Swap = &options.MemorySwap
		useMem = true
	}
	if options.MemoryReservation != 0 {
		mem.Reservation = &options.MemoryReservation
		useMem = true
	}
	if useMem {
		resources.Memory = mem
	}

	// PIDs limit
	if options.PidsLimit != nil {
		if resources.Pids == nil {
			resources.Pids = new(spec.LinuxPids)
		}
		resources.Pids.Limit = options.PidsLimit
	}

	// Blkio Weight
	if options.BlkioWeight != 0 {
		if resources.BlockIO == nil {
			resources.BlockIO = new(spec.LinuxBlockIO)
		}
		resources.BlockIO.Weight = &options.BlkioWeight
	}

	// Restart policy
	localPolicy := string(options.RestartPolicy.Name)
	restartPolicy := &localPolicy

	var restartRetries *uint
	if options.RestartPolicy.MaximumRetryCount != 0 {
		localRetries := uint(options.RestartPolicy.MaximumRetryCount)
		restartRetries = &localRetries
	}

	// Rlimits
	var rlimits []spec.POSIXRlimit
	if len(options.Ulimits) > 0 {
		ulimits := make([]string, len(options.Ulimits))
		for i, u := range options.Ulimits {
			ulimits[i] = u.String()
		}
		rlimits, err = specgenutil.GenRlimits(ulimits)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("generating rlimits: %w", err))
			return
		}
	}

	updateOptions := &entities.ContainerUpdateOptions{
		Resources:                       resources,
		ChangedHealthCheckConfiguration: &define.UpdateHealthCheckConfig{},
		RestartPolicy:                   restartPolicy,
		RestartRetries:                  restartRetries,
		Rlimits:                         rlimits,
	}

	if err := ctr.Update(updateOptions); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("updating container: %w", err))
		return
	}

	responseStruct := container.UpdateResponse{}
	utils.WriteResponse(w, http.StatusOK, responseStruct)
}
