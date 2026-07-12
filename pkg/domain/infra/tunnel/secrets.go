package tunnel

import (
	"context"
	"fmt"
	"io"

	"go.podman.io/podman/v6/pkg/bindings/secrets"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func (ic *ContainerEngine) SecretCreate(_ context.Context, name string, reader io.Reader, options entities.SecretCreateOptions) (*entities.SecretCreateReport, error) {
	opts := new(secrets.CreateOptions).
		WithDriver(options.Driver).
		WithDriverOpts(options.DriverOpts).
		WithName(name).
		WithLabels(options.Labels).
		WithReplace(options.Replace).
		WithIgnore(options.Ignore)
	created, err := secrets.Create(ic.ClientCtx, reader, opts)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (ic *ContainerEngine) SecretInspect(_ context.Context, nameOrIDs []string, options entities.SecretInspectOptions) ([]*entities.SecretInfoReport, []error, error) {
	allInspect := make([]*entities.SecretInfoReport, 0, len(nameOrIDs))
	errs := make([]error, 0, len(nameOrIDs))
	opts := new(secrets.InspectOptions).
		WithShowSecret(options.ShowSecret)

	for _, name := range nameOrIDs {
		inspected, err := secrets.Inspect(ic.ClientCtx, name, opts)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, fmt.Errorf("no secret with name or id %q: no such secret ", name))
				continue
			}
			return nil, nil, err
		}
		allInspect = append(allInspect, inspected)
	}
	return allInspect, errs, nil
}

func (ic *ContainerEngine) SecretList(_ context.Context, opts entities.SecretListRequest) ([]*entities.SecretInfoReport, error) {
	options := new(secrets.ListOptions).WithFilters(opts.Filters)
	secrs, _ := secrets.List(ic.ClientCtx, options)
	return secrs, nil
}

func (ic *ContainerEngine) SecretRm(_ context.Context, nameOrIDs []string, options entities.SecretRmOptions) ([]*entities.SecretRmReport, error) {
	allRm := make([]*entities.SecretRmReport, 0, len(nameOrIDs))
	if options.All {
		allSecrets, err := secrets.List(ic.ClientCtx, nil)
		if err != nil {
			return nil, err
		}
		for _, secret := range allSecrets {
			allRm = append(allRm, &entities.SecretRmReport{
				Err: secrets.Remove(ic.ClientCtx, secret.ID),
				ID:  secret.ID,
			})
		}
		return allRm, nil
	}
	for _, name := range nameOrIDs {
		secret, err := secrets.Inspect(ic.ClientCtx, name, nil)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, err
			}
			if errModel.ResponseCode == 404 {
				if !options.Ignore {
					allRm = append(allRm, &entities.SecretRmReport{
						Err: fmt.Errorf("no secret with name or id %q: no such secret ", name),
						ID:  "",
					})
				}
				continue
			}
		}
		allRm = append(allRm, &entities.SecretRmReport{
			Err: secrets.Remove(ic.ClientCtx, name),
			ID:  secret.ID,
		})
	}
	return allRm, nil
}

func (ic *ContainerEngine) SecretExists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	exists, err := secrets.Exists(ic.ClientCtx, nameOrID)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}
