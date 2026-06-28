//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/emulation"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage"
	"go.podman.io/storage/pkg/directory"
	"go.podman.io/storage/pkg/fileutils"
)

func (ic *ContainerEngine) Info(_ context.Context) (*define.Info, error) {
	info, err := ic.Libpod.Info()
	if err != nil {
		return nil, err
	}

	info.Host.EmulatedArchitectures = emulation.Registered()

	info.Host.RemoteSocket = &define.RemoteSocket{Path: ic.Libpod.RemoteURI()}

	// `podman system connection add` invokes podman via ssh to fill in connection string. Here
	// we are reporting the default systemd activation socket path as we cannot know if a future
	// service may be run with another URI.
	if ic.Libpod.RemoteURI() == "" {
		xdg := defaultRunPath
		if path, err := util.GetRootlessRuntimeDir(); err != nil {
			// Info is as good as we can guess...
			return info, err
		} else if path != "" {
			xdg = path
		}

		uri := url.URL{
			Scheme: "unix",
			Path:   filepath.Join(xdg, "podman", "podman.sock"),
		}
		ic.Libpod.SetRemoteURI(uri.String())
		info.Host.RemoteSocket.Path = uri.Path
	}

	// check if the unix path exits, if not unix socket we always we assume it exists, i.e. tcp socket
	path, found := strings.CutPrefix(info.Host.RemoteSocket.Path, "unix://")
	if found {
		err := fileutils.Exists(path)
		info.Host.RemoteSocket.Exists = err == nil
	} else {
		info.Host.RemoteSocket.Exists = true
	}

	return info, nil
}

// SystemPrune removes unused data from the system. Pruning pods, containers, build container, networks, volumes and images.
func (ic *ContainerEngine) SystemPrune(ctx context.Context, options entities.SystemPruneOptions) (*entities.SystemPruneReport, error) {
	systemPruneReport := new(entities.SystemPruneReport)

	if options.External {
		if options.All || options.Volume || len(options.Filters) > 0 || options.Build {
			return nil, fmt.Errorf("system prune --external cannot be combined with other options")
		}

		if err := ic.Libpod.GarbageCollect(); err != nil {
			return nil, err
		}
		return systemPruneReport, nil
	}

	filters := []string{}
	for k, v := range options.Filters {
		filters = append(filters, fmt.Sprintf("%s=%s", k, v[0]))
	}
	reclaimedSpace := (uint64)(0)

	// Prune Build Containers
	if options.Build {
		stageContainersPruneReports, err := ic.Libpod.PruneBuildContainers()
		if err != nil {
			return nil, err
		}
		reclaimedSpace += reports.PruneReportsSize(stageContainersPruneReports)
		systemPruneReport.ContainerPruneReports = append(systemPruneReport.ContainerPruneReports, stageContainersPruneReports...)
	}

	found := true
	for found {
		found = false

		// TODO: Figure out cleaner way to handle all of the different PruneOptions
		// Remove all unused pods.
		podPruneReports, err := ic.prunePodHelper(ctx)
		if err != nil {
			return nil, err
		}
		if len(podPruneReports) > 0 {
			found = true
		}

		systemPruneReport.PodPruneReport = append(systemPruneReport.PodPruneReport, podPruneReports...)

		// Remove all unused containers.
		containerPruneOptions := entities.ContainerPruneOptions{}
		containerPruneOptions.Filters = (url.Values)(options.Filters)

		containerPruneReports, err := ic.ContainerPrune(ctx, containerPruneOptions)
		if err != nil {
			return nil, err
		}

		reclaimedSpace += reports.PruneReportsSize(containerPruneReports)
		systemPruneReport.ContainerPruneReports = append(systemPruneReport.ContainerPruneReports, containerPruneReports...)

		// Remove all unused images.
		imagePruneOptions := entities.ImagePruneOptions{
			All:    options.All,
			Filter: filters,
		}

		imageEngine := ImageEngine{Libpod: ic.Libpod}
		imagePruneReports, err := imageEngine.Prune(ctx, imagePruneOptions)
		if err != nil {
			return nil, err
		}
		if len(imagePruneReports) > 0 {
			found = true
		}

		reclaimedSpace += reports.PruneReportsSize(imagePruneReports)
		systemPruneReport.ImagePruneReports = append(systemPruneReport.ImagePruneReports, imagePruneReports...)

		// Remove all unused networks.
		networkPruneOptions := entities.NetworkPruneOptions{}
		networkPruneOptions.Filters = options.Filters

		networkPruneReports, err := ic.NetworkPrune(ctx, networkPruneOptions)
		if err != nil {
			return nil, err
		}
		if len(networkPruneReports) > 0 {
			found = true
		}

		// Networks reclaimedSpace are always '0'.
		systemPruneReport.NetworkPruneReports = append(systemPruneReport.NetworkPruneReports, networkPruneReports...)

		// Remove unused volume data.
		if options.Volume {
			volumePruneOptions := entities.VolumePruneOptions{}
			volumePruneOptions.Filters = (url.Values)(options.Filters)

			if len(volumePruneOptions.Filters) == 0 {
				volumePruneOptions.Filters.Set("all", "true")
			}

			volumePruneReports, err := ic.VolumePrune(ctx, volumePruneOptions)
			if err != nil {
				return nil, err
			}
			if len(volumePruneReports) > 0 {
				found = true
			}

			reclaimedSpace += reports.PruneReportsSize(volumePruneReports)
			systemPruneReport.VolumePruneReports = append(systemPruneReport.VolumePruneReports, volumePruneReports...)
		}
	}

	systemPruneReport.ReclaimedSpace = reclaimedSpace
	return systemPruneReport, nil
}

func (ic *ContainerEngine) SystemDf(ctx context.Context, _ entities.SystemDfOptions) (*entities.SystemDfReport, error) {
	dfImages := []*entities.SystemDfImageReport{}

	imageStats, totalImageSize, err := ic.Libpod.LibimageRuntime().DiskUsage(ctx)
	if err != nil {
		return nil, err
	}

	for _, stat := range imageStats {
		report := entities.SystemDfImageReport{
			Repository: stat.Repository,
			Tag:        stat.Tag,
			ImageID:    stat.ID,
			Created:    stat.Created,
			Size:       stat.Size,
			SharedSize: stat.SharedSize,
			UniqueSize: stat.UniqueSize,
			Containers: stat.Containers,
		}
		dfImages = append(dfImages, &report)
	}

	// Get containers and iterate over them
	cons, err := ic.Libpod.GetAllContainers()
	if err != nil {
		return nil, err
	}
	dfContainers := make([]*entities.SystemDfContainerReport, 0, len(cons))
	for _, c := range cons {
		iid, _ := c.Image()
		state, err := c.State()
		if err != nil {
			if errors.Is(err, define.ErrNoSuchCtr) {
				continue
			}
			return nil, fmt.Errorf("failed to get state of container %s: %w", c.ID(), err)
		}
		conSize, err := c.RootFsSize()
		if err != nil {
			if errors.Is(err, storage.ErrContainerUnknown) || errors.Is(err, define.ErrNoSuchCtr) {
				continue
			}
			return nil, fmt.Errorf("failed to get root file system size of container %s: %w", c.ID(), err)
		}
		rwsize, err := c.RWSize()
		if err != nil {
			if errors.Is(err, storage.ErrContainerUnknown) || errors.Is(err, define.ErrNoSuchCtr) {
				continue
			}
			return nil, fmt.Errorf("failed to get read/write size of container %s: %w", c.ID(), err)
		}
		report := entities.SystemDfContainerReport{
			ContainerID:  c.ID(),
			Image:        iid,
			Command:      c.Command(),
			LocalVolumes: len(c.UserVolumes()),
			RWSize:       rwsize,
			Size:         conSize,
			Created:      c.CreatedTime(),
			Status:       state.String(),
			Names:        c.Name(),
		}
		dfContainers = append(dfContainers, &report)
	}

	// Get volumes and iterate over them
	vols, err := ic.Libpod.GetAllVolumes()
	if err != nil {
		return nil, err
	}

	dfVolumes := make([]*entities.SystemDfVolumeReport, 0, len(vols))
	for _, v := range vols {
		var reclaimableSize int64
		mountPoint, err := v.MountPoint()
		if err != nil {
			if errors.Is(err, define.ErrNoSuchVolume) {
				continue
			}
			return nil, err
		}
		if mountPoint == "" {
			// We can't get any info on this volume, as it's not
			// mounted.
			// TODO: fix this.
			continue
		}
		volSize, err := directory.Size(mountPoint)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// volume is not locked here, if the directory is missing
				// all of the sudden we must assume the volume was removed
				// in parallel and ignore it like in the case below
				continue
			}
			return nil, err
		}
		inUse, err := v.VolumeInUse()
		if err != nil {
			if errors.Is(err, define.ErrNoSuchVolume) {
				continue
			}
			return nil, err
		}
		if len(inUse) == 0 {
			reclaimableSize = volSize
		}
		report := entities.SystemDfVolumeReport{
			VolumeName:      v.Name(),
			Links:           len(inUse),
			Size:            volSize,
			ReclaimableSize: reclaimableSize,
		}
		dfVolumes = append(dfVolumes, &report)
	}

	return &entities.SystemDfReport{
		ImagesSize: totalImageSize,
		Images:     dfImages,
		Containers: dfContainers,
		Volumes:    dfVolumes,
	}, nil
}

func (ic *ContainerEngine) Reset(ctx context.Context) error {
	return ic.Libpod.Reset(ctx)
}

func (ic *ContainerEngine) Renumber(_ context.Context) error {
	return ic.Libpod.RenumberLocks()
}

func (ic *ContainerEngine) Migrate(_ context.Context, options entities.SystemMigrateOptions) error {
	return ic.Libpod.Migrate(options.NewRuntime, options.MigrateDB)
}

func unshareEnv(graphroot, runroot string) []string {
	return append(os.Environ(), "_CONTAINERS_USERNS_CONFIGURED=done",
		fmt.Sprintf("CONTAINERS_GRAPHROOT=%s", graphroot),
		fmt.Sprintf("CONTAINERS_RUNROOT=%s", runroot))
}

func (ic *ContainerEngine) Unshare(_ context.Context, args []string, options entities.SystemUnshareOptions) error {
	unshare := func() error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = unshareEnv(ic.Libpod.StorageConfig().GraphRoot, ic.Libpod.StorageConfig().RunRoot)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if options.RootlessNetNS {
		return ic.Libpod.Network().RunInRootlessNetns(unshare)
	}
	return unshare()
}

func (ic *ContainerEngine) Version(_ context.Context) (*entities.SystemVersionReport, error) {
	var report entities.SystemVersionReport
	v, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	report.Client = &v
	return &report, err
}

func (ic *ContainerEngine) Locks(_ context.Context) (*entities.LocksReport, error) {
	var report entities.LocksReport
	conflicts, held, err := ic.Libpod.LockConflicts()
	if err != nil {
		return nil, err
	}
	report.LockConflicts = conflicts
	report.LocksHeld = held
	return &report, nil
}

func (ic *ContainerEngine) SystemCheck(ctx context.Context, options entities.SystemCheckOptions) (*entities.SystemCheckReport, error) {
	report, err := ic.Libpod.SystemCheck(ctx, options)
	if err != nil {
		return nil, err
	}
	return &report, nil
}
