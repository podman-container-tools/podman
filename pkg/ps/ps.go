//go:build !remote && (linux || freebsd)

package ps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	libnetworkTypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/filters"
	psdefine "go.podman.io/podman/v6/pkg/ps/define"
	"go.podman.io/storage"
	"go.podman.io/storage/types"
)

// ExternalContainerFilter is a function to determine whether a container list is included
// in command output. Container lists to be outputted are tested using the function.
// A true return will include the container list, a false return will exclude it.
type ExternalContainerFilter func(*entities.ListContainer) bool

func GetContainerLists(runtime *libpod.Runtime, options entities.ContainerListOptions) ([]entities.ListContainer, error) {
	pss := []entities.ListContainer{}
	filterFuncs := make([]libpod.ContainerFilter, 0, len(options.Filters))
	filterExtFuncs := make([]entities.ExternalContainerFilter, 0, len(options.Filters))
	all := options.All || options.Last > 0
	if len(options.Filters) > 0 {
		for k, v := range options.Filters {
			generatedFunc, err := filters.GenerateContainerFilterFuncs(k, v, runtime)
			if err != nil && !options.External {
				return nil, err
			}
			filterFuncs = append(filterFuncs, generatedFunc)

			if options.External {
				generatedExtFunc, err := filters.GenerateExternalContainerFilterFuncs(k, v, runtime)
				if err != nil {
					return nil, err
				}
				filterExtFuncs = append(filterExtFuncs, generatedExtFunc)
			}
		}
	}

	// Docker thinks that if status is given as an input, then we should override
	// the all setting and always deal with all containers.
	if len(options.Filters["status"]) > 0 {
		all = true
	}
	if !all {
		runningOnly, err := filters.GenerateContainerFilterFuncs("status", []string{define.ContainerStateRunning.String()}, runtime)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, runningOnly)
	}

	// Load the containers with their states populated.  This speeds things
	// up considerably as we use a signel DB connection to load the
	// containers' states instead of one per container.
	//
	// This may return slightly outdated states but that's acceptable for
	// listing containers; any state is outdated the point a container lock
	// gets released.
	cons, err := runtime.GetContainers(true, filterFuncs...)
	if err != nil {
		return nil, err
	}
	if options.Last > 0 {
		// Sort the libpod containers
		sort.Sort(SortCreateTime{SortContainers: cons})
		// we should perform the lopping before we start getting
		// the expensive information on containers
		if options.Last < len(cons) {
			cons = cons[:options.Last]
		}
	}
	for _, con := range cons {
		listCon, err := ListContainerBatch(runtime, con, options)
		switch {
		// ignore both no ctr and no such pod errors as it means the ctr is gone now
		case errors.Is(err, define.ErrNoSuchCtr), errors.Is(err, define.ErrNoSuchPod):
			continue
		case err != nil:
			return nil, err
		default:
			pss = append(pss, listCon)
		}
	}

	if options.External {
		listCon, err := GetExternalContainerLists(runtime, filterExtFuncs...)
		if err != nil {
			return nil, err
		}
		pss = append(pss, listCon...)
	}

	// Sort the containers we got
	sort.Sort(SortPSCreateTime{SortPSContainers: pss})

	if options.Last > 0 {
		// only return the "last" containers caller requested
		if options.Last < len(pss) {
			pss = pss[:options.Last]
		}
	}
	return pss, nil
}

// GetExternalContainerLists returns list of external containers for e.g. created by buildah
func GetExternalContainerLists(runtime *libpod.Runtime, filterExtFuncs ...entities.ExternalContainerFilter) ([]entities.ListContainer, error) {
	pss := []*entities.ListContainer{}

	externCons, err := runtime.StorageContainers()
	if err != nil {
		return nil, err
	}

	for _, con := range externCons {
		listCon, err := ListStorageContainer(runtime, con)
		switch {
		case errors.Is(err, types.ErrLoadError):
			continue
		// Container could have been removed since listing
		case errors.Is(err, types.ErrContainerUnknown):
			continue
		case err != nil:
			return nil, err
		default:
			pss = append(pss, &listCon)
		}
	}

	filteredPss := applyExternalContainersFilters(pss, filterExtFuncs...)

	return filteredPss, nil
}

// Apply container filters on bunch of external container lists
func applyExternalContainersFilters(containersList []*entities.ListContainer, filters ...entities.ExternalContainerFilter) []entities.ListContainer {
	ctrsFiltered := make([]entities.ListContainer, 0, len(containersList))

	for _, ctr := range containersList {
		include := true
		for _, filter := range filters {
			include = include && filter(ctr)
		}

		if include {
			ctrsFiltered = append(ctrsFiltered, *ctr)
		}
	}

	return ctrsFiltered
}

// ListContainerBatch is used in ps to reduce performance hits by "batching"
// locks.
func ListContainerBatch(rt *libpod.Runtime, ctr *libpod.Container, opts entities.ContainerListOptions) (entities.ListContainer, error) {
	var (
		conConfig                               *libpod.ContainerConfig
		conState                                define.ContainerStatus
		err                                     error
		exitCode                                int32
		exited                                  bool
		pid                                     int
		size                                    *psdefine.ContainerSize
		startedTime                             time.Time
		exitedTime                              time.Time
		cgroup, ipc, mnt, net, pidns, user, uts string
		portMappings                            []libnetworkTypes.PortMapping
		networks                                []string
		healthStatus                            string
		restartCount                            uint
		podName                                 string
	)

	batchErr := ctr.Batch(func(c *libpod.Container) error {
		if opts.Sync {
			if err := c.Sync(); err != nil {
				return fmt.Errorf("unable to update container state from OCI runtime: %w", err)
			}
		}

		conConfig = c.ConfigNoCopy()
		conState, err = c.State()
		if err != nil {
			return fmt.Errorf("unable to obtain container state: %w", err)
		}

		exitCode, exited, err = c.ExitCode()
		if err != nil {
			return fmt.Errorf("unable to obtain container exit code: %w", err)
		}
		startedTime, err = c.StartedTime()
		if err != nil {
			logrus.Errorf("Getting started time for %q: %v", c.ID(), err)
		}
		exitedTime, err = c.FinishedTime()
		if err != nil {
			logrus.Errorf("Getting exited time for %q: %v", c.ID(), err)
		}

		pid, err = c.PID()
		if err != nil {
			return fmt.Errorf("unable to obtain container pid: %w", err)
		}

		portMappings, err = c.PortMappings()
		if err != nil {
			return err
		}

		networks, err = c.Networks()
		if err != nil {
			return err
		}

		healthStatus, err = c.HealthCheckStatus()
		if err != nil {
			// Do not treat a corrupted healthcheck log as a hard error.
			if !errors.Is(err, define.ErrHealthCheckLogCorrupted) {
				return err
			}
			logrus.Warnf("Container %v health check log is corrupted: %v", c.ID(), err)
		}

		restartCount, err = c.RestartCount()
		if err != nil {
			return err
		}

		if opts.Namespace {
			ctrPID := strconv.Itoa(pid)
			cgroup, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "cgroup"))
			ipc, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "ipc"))
			mnt, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "mnt"))
			net, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "net"))
			pidns, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "pid"))
			user, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "user"))
			uts, _ = getNamespaceInfo(filepath.Join("/proc", ctrPID, "ns", "uts"))
		}
		if opts.Size {
			size = new(psdefine.ContainerSize)

			rootFsSize, err := c.RootFsSize()
			if err != nil {
				logrus.Errorf("Getting root fs size for %q: %v", c.ID(), err)
			}

			rwSize, err := c.RWSize()
			if err != nil {
				logrus.Errorf("Getting rw size for %q: %v", c.ID(), err)
			}

			size.RootFsSize = rootFsSize
			size.RwSize = rwSize
		}

		if opts.Pod && len(conConfig.Pod) > 0 {
			podName, err = rt.GetPodName(conConfig.Pod)
			if err != nil {
				return fmt.Errorf("could not find container %s pod (id %s) in state: %w", conConfig.ID, conConfig.Pod, err)
			}
		}

		return nil
	})
	if batchErr != nil {
		return entities.ListContainer{}, batchErr
	}

	ps := entities.ListContainer{
		AutoRemove:   ctr.AutoRemove(),
		CIDFile:      conConfig.Spec.Annotations[define.InspectAnnotationCIDFile],
		Command:      conConfig.Command,
		Created:      conConfig.CreatedTime,
		ExitCode:     exitCode,
		Exited:       exited,
		ExitedAt:     exitedTime.Unix(),
		ExposedPorts: conConfig.ExposedPorts,
		ID:           conConfig.ID,
		Image:        conConfig.RootfsImageName,
		ImageID:      conConfig.RootfsImageID,
		IsInfra:      conConfig.IsInfra,
		Labels:       conConfig.Labels,
		Mounts:       ctr.UserVolumes(),
		Names:        []string{conConfig.Name},
		Networks:     networks,
		Pid:          pid,
		Pod:          conConfig.Pod,
		PodName:      podName,
		Ports:        portMappings,
		Restarts:     restartCount,
		Size:         size,
		StartedAt:    startedTime.Unix(),
		State:        conState.String(),
		Status:       healthStatus,
	}

	if opts.Namespace {
		ps.Namespaces = entities.ListContainerNamespaces{
			Cgroup: cgroup,
			IPC:    ipc,
			MNT:    mnt,
			NET:    net,
			PIDNS:  pidns,
			User:   user,
			UTS:    uts,
		}
	}

	return ps, nil
}

func ListStorageContainer(rt *libpod.Runtime, ctr storage.Container) (entities.ListContainer, error) {
	name := "unknown"
	if len(ctr.Names) > 0 {
		name = ctr.Names[0]
	}

	ps := entities.ListContainer{
		ID:      ctr.ID,
		Created: ctr.Created,
		ImageID: ctr.ImageID,
		State:   "storage",
		Names:   []string{name},
	}

	buildahCtr, err := rt.IsBuildahContainer(ctr.ID)
	if err != nil {
		return ps, fmt.Errorf("determining buildah container for container %s: %w", ctr.ID, err)
	}

	if buildahCtr {
		ps.Command = []string{"buildah"}
	} else {
		ps.Command = []string{"storage"}
	}

	imageName := ""
	if ctr.ImageID != "" {
		image, _, err := rt.LibimageRuntime().LookupImage(ctr.ImageID, nil)
		if err != nil {
			return ps, err
		}
		if len(image.NamesHistory()) > 0 {
			imageName = image.NamesHistory()[0]
		}
	} else if buildahCtr {
		imageName = "scratch"
	}

	ps.Image = imageName
	return ps, nil
}

func getNamespaceInfo(path string) (string, error) {
	val, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("getting info from %q: %w", path, err)
	}
	return getStrFromSquareBrackets(val), nil
}

// getStrFromSquareBrackets gets the string inside [] from a string.
func getStrFromSquareBrackets(cmd string) string {
	reg := regexp.MustCompile(`.*\[|\].*`)
	arr := strings.Split(reg.ReplaceAllLiteralString(cmd, ""), ",")
	return strings.Join(arr, ",")
}

// SortContainers helps us set-up ability to sort by createTime
type SortContainers []*libpod.Container

func (a SortContainers) Len() int      { return len(a) }
func (a SortContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type SortCreateTime struct{ SortContainers }

func (a SortCreateTime) Less(i, j int) bool {
	return a.SortContainers[i].CreatedTime().After(a.SortContainers[j].CreatedTime())
}

// SortPSContainers helps us set-up ability to sort by createTime
type SortPSContainers []entities.ListContainer

func (a SortPSContainers) Len() int      { return len(a) }
func (a SortPSContainers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

type SortPSCreateTime struct{ SortPSContainers }

func (a SortPSCreateTime) Less(i, j int) bool {
	return a.SortPSContainers[i].Created.Before(a.SortPSContainers[j].Created)
}
