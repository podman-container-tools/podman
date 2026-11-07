package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.podman.io/podman/v6/pkg/bindings/configs"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func (ic *ContainerEngine) ConfigCreate(_ context.Context, name string, reader io.Reader, options entities.ConfigCreateOptions) (*entities.ConfigCreateReport, error) {
	opts := new(configs.CreateOptions).
		WithDriver(options.Driver).
		WithDriverOpts(options.DriverOpts).
		WithName(name).
		WithLabels(options.Labels).
		WithReplace(options.Replace).
		WithIgnore(options.Ignore)
	created, err := configs.Create(ic.ClientCtx, reader, opts)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (ic *ContainerEngine) ConfigInspect(_ context.Context, nameOrIDs []string, options entities.ConfigInspectOptions) ([]*entities.ConfigInfoReport, []error, error) {
	allInspect := make([]*entities.ConfigInfoReport, 0, len(nameOrIDs))
	errs := make([]error, 0, len(nameOrIDs))
	opts := new(configs.InspectOptions).
		WithShowConfig(options.ShowConfig)

	for _, name := range nameOrIDs {
		inspected, err := configs.Inspect(ic.ClientCtx, name, opts)
		if err != nil {
			var errModel *errorhandling.ErrorModel
			if !errors.As(err, &errModel) {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, fmt.Errorf("no config with name or id %q: no such config ", name))
				continue
			}
			return nil, nil, err
		}
		allInspect = append(allInspect, inspected)
	}
	return allInspect, errs, nil
}

func (ic *ContainerEngine) ConfigList(_ context.Context, opts entities.ConfigListRequest) ([]*entities.ConfigInfoReport, error) {
	options := new(configs.ListOptions).WithFilters(opts.Filters)
	return configs.List(ic.ClientCtx, options)
}

func (ic *ContainerEngine) ConfigRm(_ context.Context, nameOrIDs []string, options entities.ConfigRmOptions) ([]*entities.ConfigRmReport, error) {
	allRm := make([]*entities.ConfigRmReport, 0, len(nameOrIDs))
	if options.All {
		allConfigs, err := configs.List(ic.ClientCtx, nil)
		if err != nil {
			return nil, err
		}
		for _, config := range allConfigs {
			allRm = append(allRm, &entities.ConfigRmReport{
				Err: configs.Remove(ic.ClientCtx, config.ID),
				ID:  config.ID,
			})
		}
		return allRm, nil
	}
	for _, name := range nameOrIDs {
		config, err := configs.Inspect(ic.ClientCtx, name, nil)
		if err != nil {
			var errModel *errorhandling.ErrorModel
			if !errors.As(err, &errModel) {
				return nil, err
			}
			if errModel.ResponseCode == 404 {
				if !options.Ignore {
					allRm = append(allRm, &entities.ConfigRmReport{
						Err: fmt.Errorf("no config with name or id %q: no such config ", name),
						ID:  "",
					})
				}
				continue
			}
			allRm = append(allRm, &entities.ConfigRmReport{Err: err})
			continue
		}
		allRm = append(allRm, &entities.ConfigRmReport{
			Err: configs.Remove(ic.ClientCtx, name),
			ID:  config.ID,
		})
	}
	return allRm, nil
}

func (ic *ContainerEngine) ConfigExists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	exists, err := configs.Exists(ic.ClientCtx, nameOrID)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}
