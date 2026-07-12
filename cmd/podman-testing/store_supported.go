//go:build (linux || freebsd) && !remote

package main

import (
	"fmt"
	"os"

	ientities "go.podman.io/podman/v6/internal/domain/entities"
	"go.podman.io/podman/v6/internal/domain/infra"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/storage"
)

var (
	globalStorageOptions storage.StoreOptions
	globalStore          storage.Store
	engineMode           = entities.ABIMode
	testingEngine        ientities.TestingEngine
)

func init() {
	if defaultStoreOptions, err := storage.DefaultStoreOptions(); err == nil {
		globalStorageOptions = defaultStoreOptions
	}
	fl := mainCmd.PersistentFlags()
	fl.StringVar(&globalStorageOptions.GraphDriverName, "storage-driver", "", "storage driver used to manage images and containers")
	fl.StringVar(&globalStorageOptions.GraphRoot, "root", "", "where images and containers will be stored")
	fl.StringVar(&globalStorageOptions.RunRoot, "runroot", "", "where volatile state information will be stored")
	fl.StringArrayVar(&globalStorageOptions.GraphDriverOptions, "storage-opt", nil, "storage driver options")
	fl.StringVar(&globalStorageOptions.ImageStore, "imagestore", "", "where to store just some parts of images")
	fl.BoolVar(&globalStorageOptions.TransientStore, "transient-store", false, "enable transient container storage")
}

func storeBefore() error {
	defaultStoreOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "selecting storage options: %v\n", err)
		return nil
	}
	globalStorageOptions = defaultStoreOptions
	store, err := storage.GetStore(globalStorageOptions)
	if err != nil {
		return err
	}
	globalStore = store
	if podmanConfig.URI != "" {
		engineMode = entities.TunnelMode
	} else {
		engineMode = entities.ABIMode
	}
	podmanConfig.EngineMode = engineMode
	return nil
}

func storeAfter() error {
	if globalStore != nil {
		_, err := globalStore.Shutdown(false)
		return err
	}
	return nil
}

func testingEngineBefore(podmanConfig *entities.PodmanConfig) (err error) {
	podmanConfig.StorageDriver = globalStorageOptions.GraphDriverName
	podmanConfig.GraphRoot = globalStorageOptions.GraphRoot
	podmanConfig.Runroot = globalStorageOptions.RunRoot
	podmanConfig.ImageStore = globalStorageOptions.ImageStore
	podmanConfig.StorageOpts = globalStorageOptions.GraphDriverOptions
	podmanConfig.TransientStore = globalStorageOptions.TransientStore
	testingEngine, err = infra.NewTestingEngine(podmanConfig)
	return err
}
