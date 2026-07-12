package healthcheck

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var runCmd = &cobra.Command{
	Use:               "run [options] CONTAINER",
	Short:             "Run the health check of a container",
	Long:              "Run the health check of a container",
	Example:           `podman healthcheck run mywebapp`,
	RunE:              run,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: common.AutocompleteContainersRunning,
}

var ignoreResult bool

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: runCmd,
		Parent:  healthCmd,
	})

	flags := runCmd.Flags()
	flags.BoolVar(&ignoreResult, "ignore-result", false,
		"Exit with code 0 regardless of healthcheck result or if the container is still in startup period")
}

func run(_ *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().HealthCheckRun(context.Background(), args[0], entities.HealthCheckOptions{})
	if err != nil {
		return err
	}
	switch response.Status {
	case define.HealthCheckUnhealthy, define.HealthCheckStarting, define.HealthCheckStopped:
		if ignoreResult {
			registry.SetExitCode(0)
		} else {
			registry.SetExitCode(1)
		}
		fmt.Println(response.Status)
	}
	return err
}
