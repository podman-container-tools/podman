package generate

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	// Command: podman _generate_
	GenerateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate structured data based on containers, pods or volumes",
		Long:  "Generate structured data (e.g., Kubernetes YAML or systemd units) based on containers, pods or volumes.",
		RunE:  validate.SubCommandExists,
	}
	containerConfig = registry.PodmanConfig().ContainersConfDefaultsRO
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: GenerateCmd,
	})
}
