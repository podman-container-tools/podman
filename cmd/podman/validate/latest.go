package validate

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

func AddLatestFlag(cmd *cobra.Command, b *bool) {
	// Initialization flag verification
	if !registry.IsRemote() {
		cmd.Flags().BoolVarP(b, "latest", "l", false,
			"Act on the latest container podman is aware of\nNot supported with the \"--remote\" flag")
	}
}
