//go:build !remote

package tunnel

import (
	"context"
	"syscall"

	"go.podman.io/podman/v6/internal/domain/entities"
)

type TestingEngine struct {
	ClientCtx context.Context
}

func (te *TestingEngine) CreateStorageLayer(_ context.Context, _ entities.CreateStorageLayerOptions) (*entities.CreateStorageLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateLayer(_ context.Context, _ entities.CreateLayerOptions) (*entities.CreateLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateLayerData(_ context.Context, _ entities.CreateLayerDataOptions) (*entities.CreateLayerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyLayer(_ context.Context, _ entities.ModifyLayerOptions) (*entities.ModifyLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) PopulateLayer(_ context.Context, _ entities.PopulateLayerOptions) (*entities.PopulateLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveStorageLayer(_ context.Context, _ entities.RemoveStorageLayerOptions) (*entities.RemoveStorageLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateImage(_ context.Context, _ entities.CreateImageOptions) (*entities.CreateImageReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateImageData(_ context.Context, _ entities.CreateImageDataOptions) (*entities.CreateImageDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveLayer(_ context.Context, _ entities.RemoveLayerOptions) (*entities.RemoveLayerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveImage(_ context.Context, _ entities.RemoveImageOptions) (*entities.RemoveImageReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveContainer(_ context.Context, _ entities.RemoveContainerOptions) (*entities.RemoveContainerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateContainer(_ context.Context, _ entities.CreateContainerOptions) (*entities.CreateContainerReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) CreateContainerData(_ context.Context, _ entities.CreateContainerDataOptions) (*entities.CreateContainerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveLayerData(_ context.Context, _ entities.RemoveLayerDataOptions) (*entities.RemoveLayerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveImageData(_ context.Context, _ entities.RemoveImageDataOptions) (*entities.RemoveImageDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) RemoveContainerData(_ context.Context, _ entities.RemoveContainerDataOptions) (*entities.RemoveContainerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyLayerData(_ context.Context, _ entities.ModifyLayerDataOptions) (*entities.ModifyLayerDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyImageData(_ context.Context, _ entities.ModifyImageDataOptions) (*entities.ModifyImageDataReport, error) {
	return nil, syscall.ENOSYS
}

func (te *TestingEngine) ModifyContainerData(_ context.Context, _ entities.ModifyContainerDataOptions) (*entities.ModifyContainerDataReport, error) {
	return nil, syscall.ENOSYS
}
