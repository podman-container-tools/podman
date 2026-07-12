//go:build !remote && (linux || freebsd)

package abi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.podman.io/image/v5/manifest"
	"go.podman.io/podman/v6/internal/domain/entities"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/storage"
	graphdriver "go.podman.io/storage/drivers"
	"go.podman.io/storage/pkg/chrootarchive"
	"go.podman.io/storage/pkg/stringid"
)

type TestingEngine struct {
	Libpod *libpod.Runtime
	Store  storage.Store
}

func (te *TestingEngine) CreateStorageLayer(_ context.Context, opts entities.CreateStorageLayerOptions) (*entities.CreateStorageLayerReport, error) {
	driver, err := te.Store.GraphDriver()
	if err != nil {
		return nil, err
	}
	id := opts.ID
	if id == "" {
		id = stringid.GenerateNonCryptoID()
	}
	if err := driver.CreateReadWrite(id, opts.Parent, &graphdriver.CreateOpts{}); err != nil {
		return nil, err
	}
	return &entities.CreateStorageLayerReport{ID: id}, nil
}

func (te *TestingEngine) CreateLayer(_ context.Context, opts entities.CreateLayerOptions) (*entities.CreateLayerReport, error) {
	layer, err := te.Store.CreateLayer(opts.ID, opts.Parent, nil, "", true, nil)
	if err != nil {
		return nil, err
	}
	return &entities.CreateLayerReport{ID: layer.ID}, nil
}

func (te *TestingEngine) CreateLayerData(_ context.Context, opts entities.CreateLayerDataOptions) (*entities.CreateLayerDataReport, error) {
	for key, data := range opts.Data {
		if err := te.Store.SetLayerBigData(opts.ID, key, bytes.NewReader(data)); err != nil {
			return nil, err
		}
	}
	return &entities.CreateLayerDataReport{}, nil
}

func (te *TestingEngine) ModifyLayer(_ context.Context, opts entities.ModifyLayerOptions) (*entities.ModifyLayerReport, error) {
	mnt, err := te.Store.Mount(opts.ID, "")
	if err != nil {
		return nil, err
	}
	modifyError := chrootarchive.UntarWithRoot(bytes.NewReader(opts.ContentsArchive), mnt, nil, mnt)
	if _, err := te.Store.Unmount(opts.ID, false); err != nil {
		return nil, err
	}
	if modifyError != nil {
		return nil, modifyError
	}
	return &entities.ModifyLayerReport{}, nil
}

func (te *TestingEngine) PopulateLayer(_ context.Context, opts entities.PopulateLayerOptions) (*entities.PopulateLayerReport, error) {
	if _, err := te.Store.ApplyDiff(opts.ID, bytes.NewReader(opts.ContentsArchive)); err != nil {
		return nil, err
	}
	return &entities.PopulateLayerReport{}, nil
}

func (te *TestingEngine) CreateImage(_ context.Context, opts entities.CreateImageOptions) (*entities.CreateImageReport, error) {
	image, err := te.Store.CreateImage(opts.ID, opts.Names, opts.Layer, "", nil)
	if err != nil {
		return nil, err
	}
	return &entities.CreateImageReport{ID: image.ID}, nil
}

func (te *TestingEngine) CreateImageData(_ context.Context, opts entities.CreateImageDataOptions) (*entities.CreateImageDataReport, error) {
	for key, data := range opts.Data {
		if err := te.Store.SetImageBigData(opts.ID, key, data, manifest.Digest); err != nil {
			return nil, err
		}
	}
	return &entities.CreateImageDataReport{}, nil
}

func (te *TestingEngine) CreateContainer(_ context.Context, opts entities.CreateContainerOptions) (*entities.CreateContainerReport, error) {
	image, err := te.Store.CreateContainer(opts.ID, opts.Names, opts.Image, opts.Layer, "", nil)
	if err != nil {
		return nil, err
	}
	return &entities.CreateContainerReport{ID: image.ID}, nil
}

func (te *TestingEngine) CreateContainerData(_ context.Context, opts entities.CreateContainerDataOptions) (*entities.CreateContainerDataReport, error) {
	for key, data := range opts.Data {
		if err := te.Store.SetContainerBigData(opts.ID, key, data); err != nil {
			return nil, err
		}
	}
	return &entities.CreateContainerDataReport{}, nil
}

func (te *TestingEngine) RemoveStorageLayer(_ context.Context, opts entities.RemoveStorageLayerOptions) (*entities.RemoveStorageLayerReport, error) {
	driver, err := te.Store.GraphDriver()
	if err != nil {
		return nil, err
	}
	if err := driver.Remove(opts.ID); err != nil {
		return nil, err
	}
	return &entities.RemoveStorageLayerReport{ID: opts.ID}, nil
}

func (te *TestingEngine) RemoveLayer(_ context.Context, opts entities.RemoveLayerOptions) (*entities.RemoveLayerReport, error) {
	if err := te.Store.Delete(opts.ID); err != nil {
		return nil, err
	}
	return &entities.RemoveLayerReport{ID: opts.ID}, nil
}

func (te *TestingEngine) RemoveImage(_ context.Context, opts entities.RemoveImageOptions) (*entities.RemoveImageReport, error) {
	if err := te.Store.Delete(opts.ID); err != nil {
		return nil, err
	}
	return &entities.RemoveImageReport{ID: opts.ID}, nil
}

func (te *TestingEngine) RemoveContainer(_ context.Context, opts entities.RemoveContainerOptions) (*entities.RemoveContainerReport, error) {
	if err := te.Store.Delete(opts.ID); err != nil {
		return nil, err
	}
	return &entities.RemoveContainerReport{ID: opts.ID}, nil
}

func (te *TestingEngine) datapath(itemType, id, key string) (string, error) {
	switch itemType {
	default:
		return "", fmt.Errorf("unknown item type %q", itemType)
	case "layer", "image", "container":
	}
	driverName := te.Store.GraphDriverName()
	graphRoot := te.Store.GraphRoot()
	datapath := filepath.Join(graphRoot, driverName+"-"+itemType+"s", id, key) // more or less accurate for keys whose names are [.a-z0-9]+
	return datapath, nil
}

func (te *TestingEngine) RemoveLayerData(_ context.Context, opts entities.RemoveLayerDataOptions) (*entities.RemoveLayerDataReport, error) {
	datapath, err := te.datapath("layer", opts.ID, opts.Key)
	if err != nil {
		return nil, err
	}
	if err = os.Remove(datapath); err != nil {
		return nil, err
	}
	return &entities.RemoveLayerDataReport{}, nil
}

func (te *TestingEngine) RemoveImageData(_ context.Context, opts entities.RemoveImageDataOptions) (*entities.RemoveImageDataReport, error) {
	datapath, err := te.datapath("image", opts.ID, opts.Key)
	if err != nil {
		return nil, err
	}
	if err = os.Remove(datapath); err != nil {
		return nil, err
	}
	return &entities.RemoveImageDataReport{}, nil
}

func (te *TestingEngine) RemoveContainerData(_ context.Context, opts entities.RemoveContainerDataOptions) (*entities.RemoveContainerDataReport, error) {
	datapath, err := te.datapath("container", opts.ID, opts.Key)
	if err != nil {
		return nil, err
	}
	if err = os.Remove(datapath); err != nil {
		return nil, err
	}
	return &entities.RemoveContainerDataReport{}, nil
}

func (te *TestingEngine) ModifyLayerData(_ context.Context, opts entities.ModifyLayerDataOptions) (*entities.ModifyLayerDataReport, error) {
	datapath, err := te.datapath("layer", opts.ID, opts.Key)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(datapath, opts.Data, 0o0600); err != nil {
		return nil, err
	}
	return &entities.ModifyLayerDataReport{}, nil
}

func (te *TestingEngine) ModifyImageData(_ context.Context, opts entities.ModifyImageDataOptions) (*entities.ModifyImageDataReport, error) {
	datapath, err := te.datapath("image", opts.ID, opts.Key)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(datapath, opts.Data, 0o0600); err != nil {
		return nil, err
	}
	return &entities.ModifyImageDataReport{}, nil
}

func (te *TestingEngine) ModifyContainerData(_ context.Context, opts entities.ModifyContainerDataOptions) (*entities.ModifyContainerDataReport, error) {
	datapath, err := te.datapath("container", opts.ID, opts.Key)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(datapath, opts.Data, 0o0600); err != nil {
		return nil, err
	}
	return &entities.ModifyContainerDataReport{}, nil
}
