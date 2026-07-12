package images

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	treeDescription = "Print layer hierarchy of an image in a tree format"
	treeCmd         = &cobra.Command{
		Use:               "tree [options] IMAGE",
		Args:              cobra.ExactArgs(1),
		Short:             treeDescription,
		Long:              treeDescription,
		RunE:              tree,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           "podman image tree alpine:latest",
	}
	treeOpts entities.ImageTreeOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: treeCmd,
		Parent:  imageCmd,
	})
	treeCmd.Flags().BoolVar(&treeOpts.WhatRequires, "whatrequires", false, "Show all child images and layers of the specified image")
}

func tree(_ *cobra.Command, args []string) error {
	results, err := registry.ImageEngine().Tree(registry.Context(), args[0], treeOpts)
	if err != nil {
		return err
	}
	fmt.Println(results.Tree)
	return nil
}
