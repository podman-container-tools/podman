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

func (ic *ContainerEngine) SecretCreate(_ context.Context, name string, reader io.Reader, options entities.SecretCreateOptions) (*entities.SecretCreateReport, error) {
	data, _ := io.ReadAll(reader)
	secretsPath := ic.Libpod.GetSecretsStorageDir()
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}

	// set defaults from config for the case they are not set by an upper layer
	// (-> i.e. tests that talk directly to the api)
	cfg, err := ic.Libpod.GetConfigNoCopy()
	if err != nil {
		return nil, err
	}
	if options.Driver == "" {
		options.Driver = cfg.Secrets.Driver
	}
	if len(options.DriverOpts) == 0 && options.Driver == cfg.Secrets.Driver {
		options.DriverOpts = cfg.Secrets.Opts
	}
	if options.DriverOpts == nil {
		options.DriverOpts = make(map[string]string)
	}

	if options.Driver == "file" {
		if _, ok := options.DriverOpts["path"]; !ok {
			options.DriverOpts["path"] = filepath.Join(secretsPath, "filedriver")
		}
	}

	storeOpts := secrets.StoreOptions{
		DriverOpts:     options.DriverOpts,
		Labels:         options.Labels,
		Replace:        options.Replace,
		IgnoreIfExists: options.Ignore,
	}

	secretID, err := manager.Store(name, data, options.Driver, storeOpts)
	if err != nil {
		return nil, err
	}

	ic.Libpod.NewSecretEvent(events.Create, secretID)

	return &entities.SecretCreateReport{
		ID: secretID,
	}, nil
}

func (ic *ContainerEngine) SecretInspect(_ context.Context, nameOrIDs []string, options entities.SecretInspectOptions) ([]*entities.SecretInfoReport, []error, error) {
	var (
		secret *secrets.Secret
		data   []byte
	)
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, nil, err
	}
	errs := make([]error, 0, len(nameOrIDs))
	reports := make([]*entities.SecretInfoReport, 0, len(nameOrIDs))
	for _, nameOrID := range nameOrIDs {
		if options.ShowSecret {
			secret, data, err = manager.LookupSecretData(nameOrID)
		} else {
			secret, err = manager.Lookup(nameOrID)
		}
		if err != nil {
			if strings.Contains(err.Error(), "no such secret") {
				errs = append(errs, err)
				continue
			} else {
				return nil, nil, fmt.Errorf("inspecting secret %s: %w", nameOrID, err)
			}
		}
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		if secret.UpdatedAt.IsZero() {
			secret.UpdatedAt = secret.CreatedAt
		}
		reports = append(reports, secretToReportWithData(*secret, string(data)))
	}

	return reports, errs, nil
}

func (ic *ContainerEngine) SecretList(_ context.Context, opts entities.SecretListRequest) ([]*entities.SecretInfoReport, error) {
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}
	secretList, err := manager.List()
	if err != nil {
		return nil, err
	}
	report := make([]*entities.SecretInfoReport, 0, len(secretList))
	for _, secret := range secretList {
		result, err := utils.IfPassesSecretsFilter(secret, opts.Filters)
		if err != nil {
			return nil, err
		}
		if result {
			report = append(report, secretToReport(secret))
		}
	}
	return report, nil
}

func (ic *ContainerEngine) SecretRm(_ context.Context, nameOrIDs []string, options entities.SecretRmOptions) ([]*entities.SecretRmReport, error) {
	var (
		err      error
		toRemove []string
		reports  = []*entities.SecretRmReport{}
	)
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}
	toRemove = nameOrIDs
	if options.All {
		allSecrs, err := manager.List()
		if err != nil {
			return nil, err
		}
		for _, secr := range allSecrs {
			toRemove = append(toRemove, secr.ID)
		}
	}
	for _, nameOrID := range toRemove {
		deletedID, err := manager.Delete(nameOrID)
		if options.Ignore && errors.Is(err, secrets.ErrNoSuchSecret) {
			continue
		}
		reports = append(reports, &entities.SecretRmReport{Err: err, ID: deletedID})
		if err == nil {
			ic.Libpod.NewSecretEvent(events.Remove, deletedID)
		}
	}

	return reports, nil
}

func (ic *ContainerEngine) SecretExists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	manager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}

	secret, err := manager.Lookup(nameOrID)
	if err != nil && !errors.Is(err, secrets.ErrNoSuchSecret) {
		return nil, err
	}

	return &entities.BoolReport{Value: secret != nil}, nil
}

func secretToReport(secret secrets.Secret) *entities.SecretInfoReport {
	return secretToReportWithData(secret, "")
}

func secretToReportWithData(secret secrets.Secret, data string) *entities.SecretInfoReport {
	return &entities.SecretInfoReport{
		ID:        secret.ID,
		CreatedAt: secret.CreatedAt,
		UpdatedAt: secret.UpdatedAt,
		Spec: entities.SecretSpec{
			Name: secret.Name,
			Driver: entities.SecretDriverSpec{
				Name:    secret.Driver,
				Options: secret.DriverOptions,
			},
			Labels: secret.Labels,
		},
		SecretData: data,
	}
}
