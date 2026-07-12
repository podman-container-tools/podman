package main

import (
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/image/v5/pkg/cli/basetls/tlsdetails"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var (
	logoutOptions = auth.LogoutOptions{}
	logoutCommand = &cobra.Command{
		Use:               "logout [options] [REGISTRY]",
		Short:             "Log out of a container registry",
		Long:              "Remove the cached username and password for the registry.",
		RunE:              logout,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: common.AutocompleteRegistries,
		Example: `podman logout quay.io
podman logout --authfile dir/auth.json quay.io
podman logout --all`,
	}
)

func init() {
	// Note that the local and the remote client behave the same: both
	// store credentials locally while the remote client will pass them
	// over the wire to the endpoint.
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: logoutCommand,
	})
	flags := logoutCommand.Flags()

	// Flags from the auth package.
	flags.AddFlagSet(auth.GetLogoutFlags(&logoutOptions))

	// Add flag completion
	completion.CompleteCommandFlags(logoutCommand, auth.GetLogoutFlagsCompletions())

	logoutOptions.Stdout = os.Stdout
	logoutOptions.AcceptUnspecifiedRegistry = true
	logoutOptions.AcceptRepositories = true
}

// Implementation of podman-logout.
func logout(_ *cobra.Command, args []string) error {
	baseTLSConfig, err := tlsdetails.BaseTLSFromOptionalFile(registry.PodmanConfig().TLSDetailsFile)
	if err != nil {
		return err
	}

	sysCtx := &types.SystemContext{
		BaseTLSConfig: baseTLSConfig.TLSConfig(),
	}
	return auth.Logout(sysCtx, &logoutOptions, args)
}
