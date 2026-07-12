package pods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

var (
	topDescription = `Specify format descriptors to alter the output.

  You may run "podman pod top -l pid pcpu seccomp" to print the process ID, the CPU percentage and the seccomp mode of each process of the latest pod.`

	topOptions = entities.PodTopOptions{}

	topCommand = &cobra.Command{
		Use:               "top [options] POD [FORMAT-DESCRIPTORS|ARGS...]",
		Short:             "Display the running processes of containers in a pod",
		Long:              topDescription,
		RunE:              top,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: common.AutocompleteTopCmd,
		Example: `podman pod top podID
podman pod top podID pid seccomp args %C
podman pod top podID -eo user,pid,comm`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: topCommand,
		Parent:  podCmd,
	})

	descriptors, err := util.GetContainerPidInformationDescriptors()
	if err == nil {
		topDescription = fmt.Sprintf("%s\n\n  Format Descriptors:\n    %s", topDescription, strings.Join(descriptors, ","))
		topCommand.Long = topDescription
	}

	flags := topCommand.Flags()
	flags.SetInterspersed(false)
	flags.BoolVar(&topOptions.ListDescriptors, "list-descriptors", false, "")
	_ = flags.MarkHidden("list-descriptors") // meant only for bash completion
	validate.AddLatestFlag(topCommand, &topOptions.Latest)
}

func top(_ *cobra.Command, args []string) error {
	if topOptions.ListDescriptors {
		descriptors, err := util.GetContainerPidInformationDescriptors()
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(descriptors, "\n"))
		return nil
	}

	if len(args) < 1 && !topOptions.Latest {
		return errors.New("you must provide the name or id of a running pod")
	}

	if topOptions.Latest {
		topOptions.Descriptors = args
	} else {
		topOptions.NameOrID = args[0]
		topOptions.Descriptors = args[1:]
	}

	topResponse, err := registry.ContainerEngine().PodTop(context.Background(), topOptions)
	if err != nil {
		return err
	}

	w, err := report.NewWriterDefault(os.Stdout)
	if err != nil {
		return err
	}

	for _, proc := range topResponse.Value {
		if _, err := fmt.Fprintln(w, proc); err != nil {
			return err
		}
	}
	return w.Flush()
}
