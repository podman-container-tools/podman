package images

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	trustDescription = `Manages which registries you trust as a source of container images based on their location.
  The location is determined by the transport and the registry host of the image.  Using this container image docker://quay.io/podman/stable as an example, docker is the transport and quay.io is the registry host.`
	trustCmd = &cobra.Command{
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
		Use:         "trust",
		Short:       "Manage container image trust policy",
		Long:        trustDescription,
		RunE:        validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: trustCmd,
		Parent:  imageCmd,
	})
}
