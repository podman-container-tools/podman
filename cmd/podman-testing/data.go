//go:build (linux || freebsd) && !remote

package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/internal/domain/entities"
)

var (
	createLayerDataDescription = `Create data for a layer in local storage.`
	createLayerDataCmd         = &cobra.Command{
		Use:               "create-layer-data [options]",
		Args:              validate.NoArgs,
		Short:             "Create data for a layer",
		Long:              createLayerDataDescription,
		RunE:              createLayerData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing create-layer-data`,
	}

	createLayerDataOpts  entities.CreateLayerDataOptions
	createLayerDataKey   string
	createLayerDataValue string
	createLayerDataFile  string

	createImageDataDescription = `Create data for an image in local storage.`
	createImageDataCmd         = &cobra.Command{
		Use:               "create-image-data [options]",
		Args:              validate.NoArgs,
		Short:             "Create data for an image",
		Long:              createImageDataDescription,
		RunE:              createImageData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing create-image-data`,
	}

	createImageDataOpts  entities.CreateImageDataOptions
	createImageDataKey   string
	createImageDataValue string
	createImageDataFile  string

	createContainerDataDescription = `Create data for a container in local storage.`
	createContainerDataCmd         = &cobra.Command{
		Use:               "create-container-data [options]",
		Args:              validate.NoArgs,
		Short:             "Create data for a container",
		Long:              createContainerDataDescription,
		RunE:              createContainerData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing create-container-data`,
	}

	createContainerDataOpts  entities.CreateContainerDataOptions
	createContainerDataKey   string
	createContainerDataValue string
	createContainerDataFile  string

	modifyLayerDataDescription = `Modify data for a layer in local storage, corrupting it.`
	modifyLayerDataCmd         = &cobra.Command{
		Use:               "modify-layer-data [options]",
		Args:              validate.NoArgs,
		Short:             "Modify data for a layer",
		Long:              modifyLayerDataDescription,
		RunE:              modifyLayerData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing modify-layer-data`,
	}

	modifyLayerDataOpts  entities.ModifyLayerDataOptions
	modifyLayerDataValue string
	modifyLayerDataFile  string

	modifyImageDataDescription = `Modify data for an image in local storage, corrupting it.`
	modifyImageDataCmd         = &cobra.Command{
		Use:               "modify-image-data [options]",
		Args:              validate.NoArgs,
		Short:             "Modify data for an image",
		Long:              modifyImageDataDescription,
		RunE:              modifyImageData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing modify-image-data`,
	}

	modifyImageDataOpts  entities.ModifyImageDataOptions
	modifyImageDataValue string
	modifyImageDataFile  string

	modifyContainerDataDescription = `Modify data for a container in local storage, corrupting it.`
	modifyContainerDataCmd         = &cobra.Command{
		Use:               "modify-container-data [options]",
		Args:              validate.NoArgs,
		Short:             "Modify data for a container",
		Long:              modifyContainerDataDescription,
		RunE:              modifyContainerData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing modify-container-data`,
	}

	modifyContainerDataOpts  entities.ModifyContainerDataOptions
	modifyContainerDataValue string
	modifyContainerDataFile  string

	removeLayerDataDescription = `Remove data from a layer in local storage, corrupting it.`
	removeLayerDataCmd         = &cobra.Command{
		Use:               "remove-layer-data [options]",
		Args:              validate.NoArgs,
		Short:             "Remove data for a layer",
		Long:              removeLayerDataDescription,
		RunE:              removeLayerData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-layer-data`,
	}

	removeLayerDataOpts entities.RemoveLayerDataOptions

	removeImageDataDescription = `Remove data from an image in local storage, corrupting it.`
	removeImageDataCmd         = &cobra.Command{
		Use:               "remove-image-data [options]",
		Args:              validate.NoArgs,
		Short:             "Remove data from an image",
		Long:              removeImageDataDescription,
		RunE:              removeImageData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-image-data`,
	}

	removeImageDataOpts entities.RemoveImageDataOptions

	removeContainerDataDescription = `Remove data from a container in local storage, corrupting it.`
	removeContainerDataCmd         = &cobra.Command{
		Use:               "remove-container-data [options]",
		Args:              validate.NoArgs,
		Short:             "Remove data from a container",
		Long:              removeContainerDataDescription,
		RunE:              removeContainerData,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-container-data`,
	}

	removeContainerDataOpts entities.RemoveContainerDataOptions
)

func init() {
	mainCmd.AddCommand(createLayerDataCmd)
	flags := createLayerDataCmd.Flags()
	flags.StringVarP(&createLayerDataOpts.ID, "layer", "i", "", "ID of the layer")
	flags.StringVarP(&createLayerDataKey, "key", "k", "", "Name of the data item")
	flags.StringVarP(&createLayerDataValue, "value", "v", "", "Value of the data item")
	flags.StringVarP(&createLayerDataFile, "file", "f", "", "File containing the data item")

	mainCmd.AddCommand(createImageDataCmd)
	flags = createImageDataCmd.Flags()
	flags.StringVarP(&createImageDataOpts.ID, "image", "i", "", "ID of the image")
	flags.StringVarP(&createImageDataKey, "key", "k", "", "Name of the data item")
	flags.StringVarP(&createImageDataValue, "value", "v", "", "Value of the data item")
	flags.StringVarP(&createImageDataFile, "file", "f", "", "File containing the data item")

	mainCmd.AddCommand(createContainerDataCmd)
	flags = createContainerDataCmd.Flags()
	flags.StringVarP(&createContainerDataOpts.ID, "container", "i", "", "ID of the container")
	flags.StringVarP(&createContainerDataKey, "key", "k", "", "Name of the data item")
	flags.StringVarP(&createContainerDataValue, "value", "v", "", "Value of the data item")
	flags.StringVarP(&createContainerDataFile, "file", "f", "", "File containing the data item")

	mainCmd.AddCommand(modifyLayerDataCmd)
	flags = modifyLayerDataCmd.Flags()
	flags.StringVarP(&modifyLayerDataOpts.ID, "layer", "i", "", "ID of the layer")
	flags.StringVarP(&modifyLayerDataOpts.Key, "key", "k", "", "Name of the data item")
	flags.StringVarP(&modifyLayerDataValue, "value", "v", "", "Value of the data item")
	flags.StringVarP(&modifyLayerDataFile, "file", "f", "", "File containing the data item")

	mainCmd.AddCommand(modifyImageDataCmd)
	flags = modifyImageDataCmd.Flags()
	flags.StringVarP(&modifyImageDataOpts.ID, "image", "i", "", "ID of the image")
	flags.StringVarP(&modifyImageDataOpts.Key, "key", "k", "", "Name of the data item")
	flags.StringVarP(&modifyImageDataValue, "value", "v", "", "Value of the data item")
	flags.StringVarP(&modifyImageDataFile, "file", "f", "", "File containing the data item")

	mainCmd.AddCommand(modifyContainerDataCmd)
	flags = modifyContainerDataCmd.Flags()
	flags.StringVarP(&modifyContainerDataOpts.ID, "container", "i", "", "ID of the container")
	flags.StringVarP(&modifyContainerDataOpts.Key, "key", "k", "", "Name of the data item")
	flags.StringVarP(&modifyContainerDataValue, "value", "v", "", "Value of the data item")
	flags.StringVarP(&modifyContainerDataFile, "file", "f", "", "File containing the data item")

	mainCmd.AddCommand(removeLayerDataCmd)
	flags = removeLayerDataCmd.Flags()
	flags.StringVarP(&removeLayerDataOpts.ID, "layer", "i", "", "ID of the layer")
	flags.StringVarP(&removeLayerDataOpts.Key, "key", "k", "", "Name of the data item")

	mainCmd.AddCommand(removeImageDataCmd)
	flags = removeImageDataCmd.Flags()
	flags.StringVarP(&removeImageDataOpts.ID, "image", "i", "", "ID of the image")
	flags.StringVarP(&removeImageDataOpts.Key, "key", "k", "", "Name of the data item")

	mainCmd.AddCommand(removeContainerDataCmd)
	flags = removeContainerDataCmd.Flags()
	flags.StringVarP(&removeContainerDataOpts.ID, "container", "i", "", "ID of the container")
	flags.StringVarP(&removeContainerDataOpts.Key, "key", "k", "", "Name of the data item")
}

func createLayerData(_ *cobra.Command, _ []string) error {
	if createLayerDataOpts.ID == "" {
		return errors.New("layer ID not specified")
	}
	if createLayerDataKey == "" {
		return errors.New("layer data name not specified")
	}
	if createLayerDataValue == "" && createLayerDataFile == "" {
		return errors.New("neither layer data value nor file specified")
	}
	createLayerDataOpts.Data = make(map[string][]byte)
	if createLayerDataValue != "" {
		createLayerDataOpts.Data[createLayerDataKey] = []byte(createLayerDataValue)
	}
	if createLayerDataFile != "" {
		buf, err := os.ReadFile(createLayerDataFile)
		if err != nil {
			return err
		}
		createLayerDataOpts.Data[createLayerDataKey] = buf
	}
	_, err := testingEngine.CreateLayerData(mainContext, createLayerDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func createImageData(_ *cobra.Command, _ []string) error {
	if createImageDataOpts.ID == "" {
		return errors.New("image ID not specified")
	}
	if createImageDataKey == "" {
		return errors.New("image data name not specified")
	}
	if createImageDataValue == "" && createImageDataFile == "" {
		return errors.New("neither image data value nor file specified")
	}
	createImageDataOpts.Data = make(map[string][]byte)
	if createImageDataValue != "" {
		createImageDataOpts.Data[createImageDataKey] = []byte(createImageDataValue)
	}
	if createImageDataFile != "" {
		d, err := os.ReadFile(createImageDataFile)
		if err != nil {
			return err
		}
		createImageDataOpts.Data[createImageDataKey] = d
	}
	_, err := testingEngine.CreateImageData(mainContext, createImageDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func createContainerData(_ *cobra.Command, _ []string) error {
	if createContainerDataOpts.ID == "" {
		return errors.New("container ID not specified")
	}
	if createContainerDataKey == "" {
		return errors.New("container data name not specified")
	}
	if createContainerDataValue == "" && createContainerDataFile == "" {
		return errors.New("neither container data value nor file specified")
	}
	createContainerDataOpts.Data = make(map[string][]byte)
	if createContainerDataValue != "" {
		createContainerDataOpts.Data[createContainerDataKey] = []byte(createContainerDataValue)
	}
	if createContainerDataFile != "" {
		d, err := os.ReadFile(createContainerDataFile)
		if err != nil {
			return err
		}
		createContainerDataOpts.Data[createContainerDataKey] = d
	}
	_, err := testingEngine.CreateContainerData(mainContext, createContainerDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func modifyLayerData(_ *cobra.Command, _ []string) error {
	if modifyLayerDataOpts.ID == "" {
		return errors.New("layer ID not specified")
	}
	if modifyLayerDataOpts.Key == "" {
		return errors.New("layer data name not specified")
	}
	if modifyLayerDataValue == "" && modifyLayerDataFile == "" {
		return errors.New("neither layer data value nor file specified")
	}
	modifyLayerDataOpts.Data = []byte(modifyLayerDataValue)
	if modifyLayerDataFile != "" {
		d, err := os.ReadFile(modifyLayerDataFile)
		if err != nil {
			return err
		}
		modifyLayerDataOpts.Data = d
	}
	_, err := testingEngine.ModifyLayerData(mainContext, modifyLayerDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func modifyImageData(_ *cobra.Command, _ []string) error {
	if modifyImageDataOpts.ID == "" {
		return errors.New("image ID not specified")
	}
	if modifyImageDataOpts.Key == "" {
		return errors.New("image data name not specified")
	}
	if modifyImageDataValue == "" && modifyImageDataFile == "" {
		return errors.New("neither image data value nor file specified")
	}
	modifyImageDataOpts.Data = []byte(modifyImageDataValue)
	if modifyImageDataFile != "" {
		d, err := os.ReadFile(modifyImageDataFile)
		if err != nil {
			return err
		}
		modifyImageDataOpts.Data = d
	}
	_, err := testingEngine.ModifyImageData(mainContext, modifyImageDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func modifyContainerData(_ *cobra.Command, _ []string) error {
	if modifyContainerDataOpts.ID == "" {
		return errors.New("container ID not specified")
	}
	if modifyContainerDataOpts.Key == "" {
		return errors.New("container data name not specified")
	}
	if modifyContainerDataValue == "" && modifyContainerDataFile == "" {
		return errors.New("neither container data value nor file specified")
	}
	modifyContainerDataOpts.Data = []byte(modifyContainerDataValue)
	if modifyContainerDataFile != "" {
		d, err := os.ReadFile(modifyContainerDataFile)
		if err != nil {
			return err
		}
		modifyContainerDataOpts.Data = d
	}
	_, err := testingEngine.ModifyContainerData(mainContext, modifyContainerDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func removeLayerData(_ *cobra.Command, _ []string) error {
	if removeLayerDataOpts.ID == "" {
		return errors.New("layer ID not specified")
	}
	if removeLayerDataOpts.Key == "" {
		return errors.New("layer data name not specified")
	}
	_, err := testingEngine.RemoveLayerData(mainContext, removeLayerDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func removeImageData(_ *cobra.Command, _ []string) error {
	if removeImageDataOpts.ID == "" {
		return errors.New("image ID not specified")
	}
	if removeImageDataOpts.Key == "" {
		return errors.New("image data name not specified")
	}
	_, err := testingEngine.RemoveImageData(mainContext, removeImageDataOpts)
	if err != nil {
		return err
	}
	return nil
}

func removeContainerData(_ *cobra.Command, _ []string) error {
	if removeContainerDataOpts.ID == "" {
		return errors.New("container ID not specified")
	}
	if removeContainerDataOpts.Key == "" {
		return errors.New("container data name not specified")
	}
	_, err := testingEngine.RemoveContainerData(mainContext, removeContainerDataOpts)
	if err != nil {
		return err
	}
	return nil
}
