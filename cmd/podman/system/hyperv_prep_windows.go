//go:build windows && remote

package system

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/machine/hyperv"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"
	"go.podman.io/podman/v6/pkg/machine/windows"
)

const (
	mountsFlag    = "mounts"
	defaultMounts = 2
)

var (
	hypervPrepDescription = `Command for Windows administrators who need to configure an host for running Hyper-V-based Podman machines.

  This command creates the required registry entries in HKEY_LOCAL_MACHINE for Hyper-V vsock communication and adds the current user to the Hyper-V Administrators group.

  After this command execution, a non-admin user can manage Hyper-V-based Podman machines (create,start,stop,delete).

  This command requires administrator privileges except when using the flag --status.`

	hypervPrepCommand = &cobra.Command{
		Use:               "hyperv-prep [options]",
		Args:              validate.NoArgs,
		Short:             "Prepare the host to run Hyper-V-based Podman machines",
		Long:              hypervPrepDescription,
		PersistentPreRunE: validate.NoOp,
		RunE:              hypervPrep,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman system hyperv-prep
podman system hyperv-prep --status
podman system hyperv-prep --reset --force`,
	}
	showStatus   bool
	resetEntries bool
	mounts       int
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: hypervPrepCommand,
		Parent:  systemCmd,
	})
	flags := hypervPrepCommand.Flags()
	flags.BoolVar(&showStatus, "status", false, "Show vsock registry entries and Hyper-V group membership status")
	flags.BoolVar(&resetEntries, "reset", false, "Remove all Podman vsock registry entries and optionally remove user from Hyper-V Administrators group")
	flags.BoolVarP(&force, "force", "f", false, "Don't ask for confirmation during reset. Valid only when used with --reset.")
	flags.IntVar(&mounts, mountsFlag, defaultMounts, "Number of vsock entries to create for mount purpose")
	hypervPrepCommand.MarkFlagsMutuallyExclusive("status", "reset", mountsFlag)
	hypervPrepCommand.MarkFlagsMutuallyExclusive("status", "force")
	_ = hypervPrepCommand.RegisterFlagCompletionFunc(mountsFlag, completion.AutocompleteNone)
}

func hypervPrep(_ *cobra.Command, _ []string) error {
	// --status can run without administrator privileges
	if showStatus {
		if err := doStatusForRegistries(); err != nil {
			return err
		}
		doStatusForGroupMembership()
		return nil
	}

	if !windows.HasAdminRights() {
		return fmt.Errorf("this command requires administrator privileges, please run in an elevated terminal")
	}

	if resetEntries {
		if err := doResetForRegistries(); err != nil {
			return err
		}
		return doResetForGroupMembership()
	}

	if err := doPreparationForRegistries(); err != nil {
		return err
	}
	return doPreparationForGroupMembership()
}

func doPreparationForRegistries() error {
	// Every VSock has a purpose (Network, Events or Fileserver) and the
	// VSocks with purpose Fileserver need as many as the machine mounts.
	entriesPerPurpose := map[vsock.HVSockPurpose]int{
		vsock.Network:    1,
		vsock.Events:     1,
		vsock.Fileserver: mounts,
	}

	var created []string
	for purpose, entries := range entriesPerPurpose {
		for range entries {
			if entry, err := vsock.NewHVSockRegistryEntry(purpose, true); err != nil {
				if errors.Is(err, vsock.ErrVSockRegistryEntryExists) {
					logrus.Infof("Registry entry for %s already exists, skipping", purpose)
					continue
				}
				return err
			} else {
				created = append(created, fmt.Sprintf("%s (port %d)", purpose.String(), entry.Port))
			}
		}
	}

	if len(created) == 0 {
		fmt.Println("All required registry entries already exist")
	} else {
		fmt.Println("Successfully created registry entries for:")
		for _, entry := range created {
			fmt.Printf("  - %s\n", entry)
		}
		fmt.Println("These entries will persist even when all machines are removed.")
	}

	return nil
}

func doPreparationForGroupMembership() error {
	if hyperv.IsHyperVAdminsGroupMember() {
		fmt.Println("User is already a member of the Hyper-V Administrators group")
		return nil
	}
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	if err := hyperv.AddUserToHyperVAdminGroup(u.Username); err != nil {
		return fmt.Errorf("failed to add user to Hyper-V Administrators group: %w", err)
	}
	fmt.Printf("\nAdded user %s to the Hyper-V Administrators group\n", u.Username)
	fmt.Println("Note: You may need to log out and log back in for the new group membership to take effect.")
	return nil
}

func doStatusForRegistries() error {
	purposes := []vsock.HVSockPurpose{vsock.Network, vsock.Events, vsock.Fileserver}
	fmt.Printf("Hyper-V vsock registry entries:\n")
	foundAny := false
	for _, purpose := range purposes {
		entries, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(purpose)
		if err != nil {
			logrus.Debugf("Error loading registry entries for %s: %v", purpose.String(), err)
			continue
		}

		if len(entries) > 0 {
			foundAny = true
			for _, entry := range entries {
				fmt.Print(entry)
			}
		}
	}
	if !foundAny {
		fmt.Println("  No vsock registry entries found.")
	}
	return nil
}

func doStatusForGroupMembership() {
	fmt.Println("Hyper-V Administrators group membership:")
	if hyperv.IsHyperVAdminsGroupMember() {
		fmt.Println("  Current user is a member")
	} else {
		fmt.Println("  Current user is NOT a member")
	}
}

func doResetForRegistries() error {
	purposes := []vsock.HVSockPurpose{vsock.Network, vsock.Events, vsock.Fileserver}
	var allEntries []*vsock.HVSockRegistryEntry
	// Load all entries for each purpose
	for _, purpose := range purposes {
		entries, err := vsock.LoadAllHVSockRegistryEntriesByPurpose(purpose)
		if err != nil {
			logrus.Debugf("Error loading registry entries for %s: %v", purpose.String(), err)
			continue
		}
		allEntries = append(allEntries, entries...)
	}
	if len(allEntries) == 0 {
		fmt.Println("No vsock registry entries found to remove.")
		return nil
	}
	if !force {
		fmt.Println("Existing VSock registry entries for Podman:")
		for _, entry := range allEntries {
			fmt.Print(entry)
		}
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Do you want to delete these entries from the Windows registry? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	var removed []string
	var removalErrors []error
	for _, entry := range allEntries {
		if err := entry.Remove(); err != nil {
			logrus.Errorf("Failed to remove registry entry %s: %v", entry.KeyName, err)
			removalErrors = append(removalErrors, fmt.Errorf("failed to remove %s: %w", entry.KeyName, err))
		} else {
			removed = append(removed, fmt.Sprintf("%s (port %d)", entry.Purpose.String(), entry.Port))
		}
	}
	if len(removed) > 0 {
		fmt.Println("Successfully removed registry entries for:")
		for _, entry := range removed {
			fmt.Printf("  - %s\n", entry)
		}
	}
	if len(removalErrors) > 0 {
		fmt.Println("\nSome entries could not be removed. See logs for details.")
		return errors.Join(removalErrors...)
	}
	return nil
}

func doResetForGroupMembership() error {
	inGroup, err := hyperv.IsCurrentUserInHyperVAdminGroup()
	if err != nil {
		return fmt.Errorf("failed to check Hyper-V Administrators group membership: %w", err)
	}
	if !inGroup {
		fmt.Println("Current user isn't a member of the Hyper-V Administrators group, it won't be removed.")
		return nil
	}
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Do you want to remove the current user from the Hyper-V Administrators group? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	if err := hyperv.RemoveUserFromHyperVAdminGroup(u.Username); err != nil {
		return fmt.Errorf("failed to remove user from Hyper-V Administrators group: %w", err)
	}
	fmt.Printf("Removed user %s from the Hyper-V Administrators group\n", u.Username)
	return nil
}
