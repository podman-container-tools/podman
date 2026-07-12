package tunnel

import (
	"context"
	"errors"
	"fmt"

	"go.podman.io/podman/v6/pkg/bindings/volumes"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func (ic *ContainerEngine) VolumeCreate(_ context.Context, opts entities.VolumeCreateOptions) (*entities.IDOrNameResponse, error) {
	response, err := volumes.Create(ic.ClientCtx, opts, nil)
	if err != nil {
		return nil, err
	}
	return &entities.IDOrNameResponse{IDOrName: response.Name}, nil
}

func (ic *ContainerEngine) VolumeRm(_ context.Context, namesOrIds []string, opts entities.VolumeRmOptions) ([]*entities.VolumeRmReport, error) {
	if opts.All {
		vols, err := volumes.List(ic.ClientCtx, nil)
		if err != nil {
			return nil, err
		}
		for _, v := range vols {
			namesOrIds = append(namesOrIds, v.Name)
		}
	}
	reports := make([]*entities.VolumeRmReport, 0, len(namesOrIds))
	for _, id := range namesOrIds {
		options := new(volumes.RemoveOptions).WithForce(opts.Force)
		if opts.Timeout != nil {
			options = options.WithTimeout(*opts.Timeout)
		}
		reports = append(reports, &entities.VolumeRmReport{
			Err: volumes.Remove(ic.ClientCtx, id, options),
			Id:  id,
		})
	}
	return reports, nil
}

func (ic *ContainerEngine) VolumeInspect(_ context.Context, namesOrIds []string, opts entities.InspectOptions) ([]*entities.VolumeInspectReport, []error, error) {
	var (
		reports = make([]*entities.VolumeInspectReport, 0, len(namesOrIds))
		errs    = []error{}
	)
	if opts.All {
		vols, err := volumes.List(ic.ClientCtx, nil)
		if err != nil {
			return nil, nil, err
		}
		for _, v := range vols {
			namesOrIds = append(namesOrIds, v.Name)
		}
	}
	for _, id := range namesOrIds {
		data, err := volumes.Inspect(ic.ClientCtx, id, nil)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, fmt.Errorf("no such volume %q", id))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, &entities.VolumeInspectReport{VolumeConfigResponse: data})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) VolumePrune(_ context.Context, opts entities.VolumePruneOptions) ([]*reports.PruneReport, error) {
	options := new(volumes.PruneOptions).WithFilters(opts.Filters).WithDryRun(opts.DryRun)
	return volumes.Prune(ic.ClientCtx, options)
}

func (ic *ContainerEngine) VolumeList(_ context.Context, opts entities.VolumeListOptions) ([]*entities.VolumeListReport, error) {
	options := new(volumes.ListOptions).WithFilters(opts.Filter)
	return volumes.List(ic.ClientCtx, options)
}

// VolumeExists checks if the given volume exists
func (ic *ContainerEngine) VolumeExists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	exists, err := volumes.Exists(ic.ClientCtx, nameOrID, nil)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{
		Value: exists,
	}, nil
}

// Volumemounted check if a given volume using plugin or filesystem is mounted or not.
// TODO: Not used and exposed to tunnel. Will be used by `export` command which is unavailable to `podman-remote`
func (ic *ContainerEngine) VolumeMounted(_ context.Context, _ string) (*entities.BoolReport, error) {
	return nil, errors.New("not implemented")
}

func (ic *ContainerEngine) VolumeMount(_ context.Context, _ []string) ([]*entities.VolumeMountReport, error) {
	return nil, errors.New("mounting volumes is not supported for remote clients")
}

func (ic *ContainerEngine) VolumeUnmount(_ context.Context, _ []string) ([]*entities.VolumeUnmountReport, error) {
	return nil, errors.New("unmounting volumes is not supported for remote clients")
}

func (ic *ContainerEngine) VolumeReload(_ context.Context) (*entities.VolumeReloadReport, error) {
	return nil, errors.New("volume reload is not supported for remote clients")
}

func (ic *ContainerEngine) VolumeExport(_ context.Context, nameOrID string, options entities.VolumeExportOptions) error {
	return volumes.Export(ic.ClientCtx, nameOrID, options.Output)
}

func (ic *ContainerEngine) VolumeImport(_ context.Context, nameOrID string, options entities.VolumeImportOptions) error {
	return volumes.Import(ic.ClientCtx, nameOrID, options.Input)
}

func (ic *ContainerEngine) VolumeRename(_ context.Context, nameOrID string, opts entities.VolumeRenameOptions) error {
	return volumes.Rename(ic.ClientCtx, nameOrID, new(volumes.RenameOptions).WithNewName(opts.NewName))
}
