package kube

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	// Command: podman _kube_
	kubeCmd = &cobra.Command{
		Use:   "kube",
		Short: "Play containers, pods or volumes from a structured file",
		Long:  "Play structured data (e.g., Kubernetes YAML) based on containers, pods or volumes.",
		RunE:  validate.SubCommandExists,
	}
	// Command: podman _play_
	playKubeParentCmd = &cobra.Command{
		Use:    "play",
		Short:  "Play containers, pods or volumes from a structured file",
		Long:   "Play structured data (e.g., Kubernetes YAML) based on containers, pods or volumes.",
		Hidden: true,
		RunE:   validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeCmd,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: playKubeParentCmd,
	})
}
