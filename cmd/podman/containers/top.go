package containers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

var (
	topDescription = `Display the running processes of a container.

  The top command extends the ps(1) compatible AIX descriptors with container-specific ones as shown below.  In the presence of ps(1) specific flags (e.g, -eo), Podman will execute ps(1) inside the container.
`
	topOptions = entities.TopOptions{}

	topCommand = &cobra.Command{
		Use:               "top [options] CONTAINER [FORMAT-DESCRIPTORS|ARGS...]",
		Short:             "Display the running processes of a container",
		Long:              topDescription,
		RunE:              top,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: common.AutocompleteTopCmd,
		Example: `podman top ctrID
podman top ctrID pid seccomp args %C
podman top ctrID -eo user,pid,comm`,
	}

	containerTopCommand = &cobra.Command{
		Use:               topCommand.Use,
		Short:             topCommand.Short,
		Long:              topCommand.Long,
		RunE:              topCommand.RunE,
		ValidArgsFunction: topCommand.ValidArgsFunction,
		Example: `podman container top ctrID
podman container top ctrID pid seccomp args %C
podman container top ctrID -eo user,pid,comm`,
	}
)

func topFlags(flags *pflag.FlagSet) {
	flags.SetInterspersed(false)
	flags.BoolVar(&topOptions.ListDescriptors, "list-descriptors", false, "")
	_ = flags.MarkHidden("list-descriptors") // meant only for bash completion
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: topCommand,
	})
	topFlags(topCommand.Flags())
	validate.AddLatestFlag(topCommand, &topOptions.Latest)

	descriptors, err := util.GetContainerPidInformationDescriptors()
	if err == nil {
		topDescription = fmt.Sprintf("%s\n\n  Format Descriptors:\n    %s", topDescription, strings.Join(descriptors, ","))
		topCommand.Long = topDescription
	}

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerTopCommand,
		Parent:  containerCmd,
	})
	topFlags(containerTopCommand.Flags())
	validate.AddLatestFlag(containerTopCommand, &topOptions.Latest)
}

func top(cmd *cobra.Command, args []string) error {
	if topOptions.ListDescriptors {
		descriptors, err := util.GetContainerPidInformationDescriptors()
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(descriptors, "\n"))
		return nil
	}

	if len(args) < 1 && !topOptions.Latest {
		return errors.New("you must provide the name or id of a running container")
	}

	if topOptions.Latest {
		topOptions.Descriptors = args
	} else {
		topOptions.NameOrID = strings.TrimPrefix(args[0], "/")
		topOptions.Descriptors = args[1:]
	}

	topResponse, err := registry.ContainerEngine().ContainerTop(context.Background(), topOptions)
	if err != nil {
		return err
	}

	rpt := report.New(os.Stdout, cmd.Name()).Init(os.Stdout, 12, 2, 2, ' ', 0)
	defer rpt.Flush()

	for _, proc := range topResponse.Value {
		if _, err := fmt.Fprintln(rpt.Writer(), proc); err != nil {
			return err
		}
	}
	return nil
}
