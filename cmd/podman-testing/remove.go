//go:build (linux || freebsd) && !remote

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/internal/domain/entities"
)

var (
	removeStorageLayerDescription = `Remove an unmanaged layer in local storage, potentially corrupting it.`
	removeStorageLayerCmd         = &cobra.Command{
		Use:               "remove-storage-layer [options]",
		Args:              validate.NoArgs,
		Short:             "Remove an unmanaged layer",
		Long:              removeStorageLayerDescription,
		RunE:              removeStorageLayer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-storage-layer`,
	}

	removeStorageLayerOpts entities.RemoveStorageLayerOptions

	removeLayerDescription = `Remove a layer in local storage, potentially corrupting it.`
	removeLayerCmd         = &cobra.Command{
		Use:               "remove-layer [options]",
		Args:              validate.NoArgs,
		Short:             "Remove a layer",
		Long:              removeLayerDescription,
		RunE:              removeLayer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-layer`,
	}

	removeLayerOpts entities.RemoveLayerOptions

	removeImageDescription = `Remove an image in local storage, potentially corrupting it.`
	removeImageCmd         = &cobra.Command{
		Use:               "remove-image [options]",
		Args:              validate.NoArgs,
		Short:             "Remove an image",
		Long:              removeImageDescription,
		RunE:              removeImage,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-image`,
	}

	removeImageOpts entities.RemoveImageOptions

	removeContainerDescription = `Remove a container in local storage, potentially corrupting it.`
	removeContainerCmd         = &cobra.Command{
		Use:               "remove-container [options]",
		Args:              validate.NoArgs,
		Short:             "Remove an container",
		Long:              removeContainerDescription,
		RunE:              removeContainer,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman-testing remove-container`,
	}

	removeContainerOpts entities.RemoveContainerOptions
)

func init() {
	mainCmd.AddCommand(removeStorageLayerCmd)
	flags := removeStorageLayerCmd.Flags()
	flags.StringVarP(&removeStorageLayerOpts.ID, "layer", "i", "", "ID of the layer to remove")

	mainCmd.AddCommand(removeLayerCmd)
	flags = removeLayerCmd.Flags()
	flags.StringVarP(&removeLayerOpts.ID, "layer", "i", "", "ID of the layer to remove")

	mainCmd.AddCommand(removeImageCmd)
	flags = removeImageCmd.Flags()
	flags.StringVarP(&removeImageOpts.ID, "image", "i", "", "ID of the image to remove")

	mainCmd.AddCommand(removeContainerCmd)
	flags = removeContainerCmd.Flags()
	flags.StringVarP(&removeContainerOpts.ID, "container", "i", "", "ID of the container to remove")
}

func removeStorageLayer(_ *cobra.Command, _ []string) error {
	results, err := testingEngine.RemoveStorageLayer(mainContext, removeStorageLayerOpts)
	if err != nil {
		return err
	}
	fmt.Println(results.ID)
	return nil
}

func removeLayer(_ *cobra.Command, _ []string) error {
	results, err := testingEngine.RemoveLayer(mainContext, removeLayerOpts)
	if err != nil {
		return err
	}
	fmt.Println(results.ID)
	return nil
}

func removeImage(_ *cobra.Command, _ []string) error {
	results, err := testingEngine.RemoveImage(mainContext, removeImageOpts)
	if err != nil {
		return err
	}
	fmt.Println(results.ID)
	return nil
}

func removeContainer(_ *cobra.Command, _ []string) error {
	results, err := testingEngine.RemoveContainer(mainContext, removeContainerOpts)
	if err != nil {
		return err
	}
	fmt.Println(results.ID)
	return nil
}
