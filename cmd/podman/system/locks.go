package system

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var locksCommand = &cobra.Command{
	Use:    "locks",
	Short:  "Debug Libpod's use of locks, identifying any potential conflicts",
	Args:   validate.NoArgs,
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runLocks()
	},
	Example: "podman system locks",
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: locksCommand,
		Parent:  systemCmd,
	})
}

func runLocks() error {
	report, err := registry.ContainerEngine().Locks(registry.Context())
	if err != nil {
		return err
	}

	for lockNum, objects := range report.LockConflicts {
		fmt.Printf("Lock %d is in use by the following\n:", lockNum)
		for _, obj := range objects {
			fmt.Printf("\t%s\n", obj)
		}
	}

	if len(report.LockConflicts) > 0 {
		fmt.Printf("\nLock conflicts have been detected. Recommend immediate use of `podman system renumber` to resolve.\n\n")
	} else {
		fmt.Printf("\nNo lock conflicts have been detected.\n\n")
	}

	for _, lockNum := range report.LocksHeld {
		fmt.Printf("Lock %d is presently being held\n", lockNum)
	}

	return nil
}
