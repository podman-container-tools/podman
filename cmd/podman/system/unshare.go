package system

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
)

var (
	unshareOptions     = entities.SystemUnshareOptions{}
	unshareDescription = "Runs a command in a modified user namespace."
	unshareCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "unshare [options] [COMMAND [ARG...]]",
		Short:             "Run a command in a modified user namespace",
		Long:              unshareDescription,
		RunE:              unshare,
		ValidArgsFunction: completion.AutocompleteDefault,
		Example: `podman unshare id
podman unshare cat /proc/self/uid_map
podman unshare podman-script.sh`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: unshareCommand,
	})
	flags := unshareCommand.Flags()
	flags.SetInterspersed(false)
	flags.BoolVar(&unshareOptions.RootlessNetNS, "rootless-netns", false, "Join the rootless network namespace used for netavark networking")
}

func unshare(_ *cobra.Command, args []string) error {
	if isRootless := rootless.IsRootless(); !isRootless {
		return errors.New("please use unshare with rootless")
	}
	// exec the specified command, if there is one
	if len(args) < 1 {
		// try to exec the shell, if one's set
		shell, shellSet := os.LookupEnv("SHELL")
		if !shellSet {
			return errors.New("no command specified and no $SHELL specified")
		}
		args = []string{shell}
	}

	err := registry.ContainerEngine().Unshare(registry.Context(), args, unshareOptions)
	return utils.HandleOSExecError(err)
}
