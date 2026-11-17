//go:build !remote

package entities

import (
	"context"
)

type TestingEngine interface { //nolint:interfacebloat
	CreateStorageLayer(ctx context.Context, opts CreateStorageLayerOptions) (*CreateStorageLayerReport, error)
	CreateLayer(ctx context.Context, opts CreateLayerOptions) (*CreateLayerReport, error)
	CreateLayerData(ctx context.Context, opts CreateLayerDataOptions) (*CreateLayerDataReport, error)
	CreateImage(ctx context.Context, opts CreateImageOptions) (*CreateImageReport, error)
	CreateImageData(ctx context.Context, opts CreateImageDataOptions) (*CreateImageDataReport, error)
	CreateContainer(ctx context.Context, opts CreateContainerOptions) (*CreateContainerReport, error)
	CreateContainerData(ctx context.Context, opts CreateContainerDataOptions) (*CreateContainerDataReport, error)
	ModifyLayer(ctx context.Context, opts ModifyLayerOptions) (*ModifyLayerReport, error)
	PopulateLayer(ctx context.Context, opts PopulateLayerOptions) (*PopulateLayerReport, error)
	RemoveStorageLayer(ctx context.Context, opts RemoveStorageLayerOptions) (*RemoveStorageLayerReport, error)
	RemoveLayer(ctx context.Context, opts RemoveLayerOptions) (*RemoveLayerReport, error)
	RemoveImage(ctx context.Context, opts RemoveImageOptions) (*RemoveImageReport, error)
	RemoveContainer(ctx context.Context, opts RemoveContainerOptions) (*RemoveContainerReport, error)
	RemoveLayerData(ctx context.Context, opts RemoveLayerDataOptions) (*RemoveLayerDataReport, error)
	RemoveImageData(ctx context.Context, opts RemoveImageDataOptions) (*RemoveImageDataReport, error)
	RemoveContainerData(ctx context.Context, opts RemoveContainerDataOptions) (*RemoveContainerDataReport, error)
	ModifyLayerData(ctx context.Context, opts ModifyLayerDataOptions) (*ModifyLayerDataReport, error)
	ModifyImageData(ctx context.Context, opts ModifyImageDataOptions) (*ModifyImageDataReport, error)
	ModifyContainerData(ctx context.Context, opts ModifyContainerDataOptions) (*ModifyContainerDataReport, error)
}
