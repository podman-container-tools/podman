package containers

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	attachDescription = "The podman attach command allows you to attach to a running container using the container's ID or name, either to view its ongoing output or to control it interactively."
	attachCommand     = &cobra.Command{
		Use:               "attach [options] CONTAINER",
		Short:             "Attach to a running container",
		Long:              attachDescription,
		RunE:              attach,
		Args:              validate.IDOrLatestArgs,
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman attach ctrID
podman attach 1234
podman attach --no-stdin foobar`,
	}

	containerAttachCommand = &cobra.Command{
		Use:               attachCommand.Use,
		Short:             attachCommand.Short,
		Long:              attachCommand.Long,
		RunE:              attachCommand.RunE,
		Args:              validate.IDOrLatestArgs,
		ValidArgsFunction: attachCommand.ValidArgsFunction,
		Example: `podman container attach ctrID
podman container attach 1234
podman container attach --no-stdin foobar`,
	}
)

var attachOpts entities.AttachOptions

func attachFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&attachOpts.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	flags.BoolVar(&attachOpts.NoStdin, "no-stdin", false, "Do not attach STDIN. The default is false")
	flags.BoolVar(&attachOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: attachCommand,
	})
	attachFlags(attachCommand)
	validate.AddLatestFlag(attachCommand, &attachOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerAttachCommand,
		Parent:  containerCmd,
	})
	attachFlags(containerAttachCommand)
	validate.AddLatestFlag(containerAttachCommand, &attachOpts.Latest)
}

func attach(_ *cobra.Command, args []string) error {
	if len(args) > 1 || (len(args) == 0 && !attachOpts.Latest) {
		return errors.New("attach requires the name or id of one running container or the latest flag")
	}

	var name string
	if len(args) > 0 {
		name = strings.TrimPrefix(args[0], "/")
	}
	attachOpts.Stdin = os.Stdin
	if attachOpts.NoStdin {
		attachOpts.Stdin = nil
	}
	attachOpts.Stdout = os.Stdout
	attachOpts.Stderr = os.Stderr
	return registry.ContainerEngine().ContainerAttach(registry.Context(), name, attachOpts)
}
