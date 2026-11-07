//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"go.podman.io/common/pkg/secrets"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/utils"
)

func (ic *ContainerEngine) ConfigCreate(_ context.Context, name string, reader io.Reader, options entities.ConfigCreateOptions) (*entities.ConfigCreateReport, error) {
	if options.Ignore && options.Replace {
		return nil, errors.New("cannot use --ignore and --replace flags together")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	manager, err := ic.Libpod.ConfigsManager()
	if err != nil {
		return nil, err
	}
	if options.Driver == "" {
		options.Driver = "file"
	}
	if options.DriverOpts == nil {
		options.DriverOpts = map[string]string{}
	}
	if options.Driver == "file" {
		if _, found := options.DriverOpts["path"]; !found {
			options.DriverOpts["path"] = filepath.Join(ic.Libpod.GetConfigsStorageDir(), "filedriver")
		}
	}
	storeOpts := secrets.StoreOptions{
		DriverOpts:     options.DriverOpts,
		Labels:         options.Labels,
		Replace:        options.Replace,
		IgnoreIfExists: options.Ignore,
	}
	id, err := manager.Store(name, data, options.Driver, storeOpts)
	if err != nil {
		if strings.Contains(err.Error(), "secret name in use") {
			return nil, fmt.Errorf("%s: config name in use", name)
		}
		if strings.Contains(err.Error(), "invalid secret name") {
			return nil, errors.New(strings.ReplaceAll(err.Error(), "secret", "config"))
		}
		return nil, err
	}
	ic.Libpod.NewConfigEvent(events.Create, id)
	return &entities.ConfigCreateReport{ID: id}, nil
}

func (ic *ContainerEngine) ConfigInspect(_ context.Context, namesOrIDs []string, options entities.ConfigInspectOptions) ([]*entities.ConfigInfoReport, []error, error) {
	manager, err := ic.Libpod.ConfigsManager()
	if err != nil {
		return nil, nil, err
	}
	reports := make([]*entities.ConfigInfoReport, 0, len(namesOrIDs))
	errs := make([]error, 0, len(namesOrIDs))
	for _, nameOrID := range namesOrIDs {
		var config *secrets.Secret
		var data []byte
		if options.ShowConfig {
			config, data, err = manager.LookupSecretData(nameOrID)
		} else {
			config, err = manager.Lookup(nameOrID)
		}
		if err != nil {
			if errors.Is(err, secrets.ErrNoSuchSecret) {
				errs = append(errs, fmt.Errorf("no config with name or id %q: %w", nameOrID, entities.ErrNoSuchConfig))
				continue
			}
			return nil, nil, fmt.Errorf("inspecting config %s: %w", nameOrID, err)
		}
		if config.Labels == nil {
			config.Labels = map[string]string{}
		}
		if config.UpdatedAt.IsZero() {
			config.UpdatedAt = config.CreatedAt
		}
		reports = append(reports, &entities.ConfigInfoReport{
			ID:        config.ID,
			CreatedAt: config.CreatedAt,
			UpdatedAt: config.UpdatedAt,
			Spec: entities.ConfigSpec{
				Name:   config.Name,
				Driver: entities.ConfigDriverSpec{Name: config.Driver, Options: config.DriverOptions},
				Labels: config.Labels,
			},
			ConfigData: string(data),
		})
	}
	return reports, errs, nil
}

func (ic *ContainerEngine) ConfigList(_ context.Context, opts entities.ConfigListRequest) ([]*entities.ConfigInfoReport, error) {
	manager, err := ic.Libpod.ConfigsManager()
	if err != nil {
		return nil, err
	}
	configs, err := manager.List()
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.ConfigInfoReport, 0, len(configs))
	for _, config := range configs {
		passes, err := utils.IfPassesSecretsFilter(config, opts.Filters)
		if err != nil {
			return nil, err
		}
		if passes {
			if config.Labels == nil {
				config.Labels = map[string]string{}
			}
			if config.UpdatedAt.IsZero() {
				config.UpdatedAt = config.CreatedAt
			}
			reports = append(reports, &entities.ConfigInfoReport{
				ID:        config.ID,
				CreatedAt: config.CreatedAt,
				UpdatedAt: config.UpdatedAt,
				Spec:      entities.ConfigSpec{Name: config.Name, Driver: entities.ConfigDriverSpec{Name: config.Driver, Options: config.DriverOptions}, Labels: config.Labels},
			})
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ConfigRm(_ context.Context, namesOrIDs []string, options entities.ConfigRmOptions) ([]*entities.ConfigRmReport, error) {
	manager, err := ic.Libpod.ConfigsManager()
	if err != nil {
		return nil, err
	}
	toRemove := namesOrIDs
	if options.All {
		configs, err := manager.List()
		if err != nil {
			return nil, err
		}
		for _, config := range configs {
			toRemove = append(toRemove, config.ID)
		}
	}
	reports := make([]*entities.ConfigRmReport, 0, len(toRemove))
	for _, nameOrID := range toRemove {
		id, err := manager.Delete(nameOrID)
		if errors.Is(err, secrets.ErrNoSuchSecret) {
			err = fmt.Errorf("no config with name or id %q: %w", nameOrID, entities.ErrNoSuchConfig)
		}
		if options.Ignore && errors.Is(err, entities.ErrNoSuchConfig) {
			continue
		}
		reports = append(reports, &entities.ConfigRmReport{ID: id, Err: err})
		if err == nil {
			ic.Libpod.NewConfigEvent(events.Remove, id)
		}
	}
	return reports, nil
}

func (ic *ContainerEngine) ConfigExists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	manager, err := ic.Libpod.ConfigsManager()
	if err != nil {
		return nil, err
	}
	config, err := manager.Lookup(nameOrID)
	if err != nil && !errors.Is(err, secrets.ErrNoSuchSecret) {
		return nil, err
	}
	return &entities.BoolReport{Value: config != nil}, nil
}
