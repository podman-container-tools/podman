package kube

import (
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

type downKubeOptions struct {
	Force bool
}

var (
	downDescription = `Reads in a structured file of Kubernetes YAML.

  Removes pods that have been based on the Kubernetes kind described in the YAML.`

	downCmd = &cobra.Command{
		Use:               "down [options] [KUBEFILE [KUBEFILE...]]|-",
		Short:             "Remove pods based on Kubernetes YAML",
		Long:              downDescription,
		RunE:              down,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman kube down nginx.yml
cat nginx.yml | podman kube down -
podman kube down https://example.com/nginx.yml`,
	}

	downOptions = downKubeOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: downCmd,
		Parent:  kubeCmd,
	})
	downFlags(downCmd)
}

func downFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)

	flags.BoolVar(&downOptions.Force, "force", false, "remove volumes")
}

func down(_ *cobra.Command, args []string) error {
	reader, err := readerFromArgs(args)
	if err != nil {
		return err
	}
	return teardown(reader, entities.PlayKubeDownOptions{Force: downOptions.Force})
}
