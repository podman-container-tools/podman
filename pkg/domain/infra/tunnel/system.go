package tunnel

import (
	"context"
	"errors"

	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings/system"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) Info(_ context.Context) (*define.Info, error) {
	return system.Info(ic.ClientCtx, nil)
}

func (ic *ContainerEngine) SetupRootless(_ context.Context, _ bool, _ string) error {
	panic(errors.New("rootless engine mode is not supported when tunneling"))
}

// SystemPrune prunes unused data from the system.
func (ic *ContainerEngine) SystemPrune(_ context.Context, opts entities.SystemPruneOptions) (*entities.SystemPruneReport, error) {
	options := new(system.PruneOptions).WithAll(opts.All).WithVolumes(opts.Volume).WithFilters(opts.Filters).WithExternal(opts.External).WithBuild(opts.Build)
	return system.Prune(ic.ClientCtx, options)
}

func (ic *ContainerEngine) SystemCheck(_ context.Context, opts entities.SystemCheckOptions) (*entities.SystemCheckReport, error) {
	options := new(system.CheckOptions).WithQuick(opts.Quick).WithRepair(opts.Repair).WithRepairLossy(opts.RepairLossy)
	if opts.UnreferencedLayerMaximumAge != nil {
		duration := *opts.UnreferencedLayerMaximumAge
		options = options.WithUnreferencedLayerMaximumAge(duration.String())
	}
	return system.Check(ic.ClientCtx, options)
}

func (ic *ContainerEngine) Migrate(_ context.Context, _ entities.SystemMigrateOptions) error {
	return errors.New("runtime migration is not supported on remote clients")
}

func (ic *ContainerEngine) Renumber(_ context.Context) error {
	return errors.New("lock renumbering is not supported on remote clients")
}

func (ic *ContainerEngine) Reset(_ context.Context) error {
	return errors.New("system reset is not supported on remote clients")
}

func (ic *ContainerEngine) SystemDf(_ context.Context, _ entities.SystemDfOptions) (*entities.SystemDfReport, error) {
	return system.DiskUsage(ic.ClientCtx, nil)
}

func (ic *ContainerEngine) Unshare(_ context.Context, _ []string, _ entities.SystemUnshareOptions) error {
	return errors.New("unshare is not supported on remote clients")
}

func (ic *ContainerEngine) Version(_ context.Context) (*entities.SystemVersionReport, error) {
	return system.Version(ic.ClientCtx, nil)
}

func (ic *ContainerEngine) Locks(_ context.Context) (*entities.LocksReport, error) {
	return nil, errors.New("locks is not supported on remote clients")
}
