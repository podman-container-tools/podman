//go:build !remote

package system

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/buildah/pkg/volumes"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	systemResetDescription = `Reset podman storage back to default state

  All containers will be stopped and removed, and all images, volumes, networks and container content will be removed.
  This command does not restart podman.service and podman.socket systemd units. You may need to manually restart it after running this command.
`
	systemResetCommand = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "reset [options]",
		Args:              validate.NoArgs,
		Short:             "Reset podman storage",
		Long:              systemResetDescription,
		Run:               reset,
		ValidArgsFunction: completion.AutocompleteNone,
	}

	forceFlag bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemResetCommand,
		Parent:  systemCmd,
	})
	flags := systemResetCommand.Flags()
	flags.BoolVarP(&forceFlag, "force", "f", false, "Do not prompt for confirmation")
}

func reset(_ *cobra.Command, _ []string) {
	// Get all the external containers in use
	listCtn, err := registry.ContainerEngine().ContainerListExternal(registry.Context())
	if err != nil {
		logrus.Error(err)
	}
	// Prompt for confirmation if --force is not set
	if !forceFlag {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println(`WARNING! This will remove:
        - all containers
        - all pods
        - all images
        - all networks
        - all build cache
        - all machines
        - all volumes`)

		info, _ := registry.ContainerEngine().Info(registry.Context())
		// lets not hard fail in case of an error
		if info != nil {
			fmt.Printf("        - the graphRoot directory: %q\n", info.Store.GraphRoot)
			fmt.Printf("        - the runRoot directory: %q\n", info.Store.RunRoot)
		}

		if len(listCtn) > 0 {
			fmt.Println(`WARNING! The following external containers will be purged:`)
			// print first 12 characters of ID and first configured name alias
			for _, externalCtn := range listCtn {
				fmt.Printf("	- %s (%s)\n", externalCtn.ID[0:12], externalCtn.Names[0])
			}
		}
		fmt.Print(`Are you sure you want to continue? [y/N] `)
		answer, err := reader.ReadString('\n')
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}
		if strings.ToLower(answer)[0] != 'y' {
			os.Exit(0)
		}
	}

	// Clean build cache if any
	if err := volumes.CleanCacheMount(); err != nil {
		logrus.Error(err)
	}

	// ContainerEngine() is unusable and shut down after this.
	if err := registry.ContainerEngine().Reset(registry.Context()); err != nil {
		logrus.Error(err)
	}

	// Shutdown podman-machine and delete all machine files
	if err := resetMachine(); err != nil {
		logrus.Error(err)
	}

	os.Exit(0)
}
