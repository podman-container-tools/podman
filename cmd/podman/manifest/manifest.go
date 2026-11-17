package manifest

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	manifestDescription = "Creates, modifies, and pushes manifest lists and image indexes."
	manifestCmd         = &cobra.Command{
		Use:   "manifest",
		Short: "Manipulate manifest lists and image indexes",
		Long:  manifestDescription,
		RunE:  validate.SubCommandExists,
		Example: `podman manifest add mylist:v1.11 image:v1.11-amd64
podman manifest create localhost/list
podman manifest inspect localhost/list
podman manifest annotate --annotation left=right mylist:v1.11 sha256:15352d97781ffdf357bf3459c037be3efac4133dc9070c2dce7eca7c05c3e736
podman manifest push mylist:v1.11 docker://quay.io/myuser/image:v1.11
podman manifest remove mylist:v1.11 sha256:15352d97781ffdf357bf3459c037be3efac4133dc9070c2dce7eca7c05c3e736
podman manifest rm mylist:v1.11`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: manifestCmd,
	})
}
