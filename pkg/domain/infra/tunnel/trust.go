package tunnel

import (
	"context"
	"errors"

	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ir *ImageEngine) ShowTrust(_ context.Context, _ []string, _ entities.ShowTrustOptions) (*entities.ShowTrustReport, error) {
	return nil, errors.New("not implemented")
}

func (ir *ImageEngine) SetTrust(_ context.Context, _ []string, _ entities.SetTrustOptions) error {
	return errors.New("not implemented")
}
