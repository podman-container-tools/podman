//go:build amd64 || arm64

package machine

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/shim"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

var sshCmd = &cobra.Command{
	Use:               "ssh [options] [NAME] [COMMAND [ARG ...]]",
	Short:             "SSH into an existing machine",
	Long:              "SSH into a managed virtual machine ",
	PersistentPreRunE: machinePreRunE,
	RunE:              ssh,
	Example: `podman machine ssh podman-machine-default
podman machine ssh myvm echo hello`,
	ValidArgsFunction: autocompleteMachineSSH,
}

var sshOpts machine.SSHOptions

func init() {
	sshCmd.Flags().SetInterspersed(false)
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: sshCmd,
		Parent:  machineCmd,
	})
	flags := sshCmd.Flags()
	usernameFlagName := "username"
	flags.StringVar(&sshOpts.Username, usernameFlagName, "", "Username to use when ssh-ing into the VM.")
	_ = sshCmd.RegisterFlagCompletionFunc(usernameFlagName, completion.AutocompleteNone)
}

func ssh(_ *cobra.Command, args []string) error {
	var (
		err        error
		exists     bool
		mc         *vmconfigs.MachineConfig
		vmProvider vmconfigs.VMProvider
	)

	// Set the VM to default
	vmName := defaultMachineName
	// If len is greater than 0, it means we may have been
	// provided the VM name.  If so, we check.  The VM name,
	// if provided, must be in args[0].
	if len(args) > 0 {
		// note: previous incantations of this up by a specific name
		// and errors were ignored.  this error is not ignored because
		// it implies podman cannot read its machine files, which is bad
		mc, vmProvider, err = shim.VMExists(args[0])
		if err != nil {
			var vmNotExistsErr *define.ErrVMDoesNotExist
			if !errors.As(err, &vmNotExistsErr) {
				return err
			}
			sshOpts.Args = append(sshOpts.Args, args[0])
		} else {
			vmName = args[0]
			exists = true
		}
	}

	// If len is greater than 1, it means we might have been
	// given a vmname and args or just args
	if len(args) > 1 {
		if exists {
			sshOpts.Args = args[1:]
		} else {
			sshOpts.Args = args
		}
	}

	// If the machine config was not loaded earlier, we load it now
	if mc == nil {
		mc, vmProvider, err = shim.VMExists(vmName)
		if err != nil {
			return err
		}
	}
	state, err := vmProvider.State(mc, false)
	if err != nil {
		return err
	}
	if state != define.Running {
		return fmt.Errorf("vm %q is not running", mc.Name)
	}

	if sshOpts.Username == "" {
		if mc.HostUser.Rootful {
			sshOpts.Username = "root"
		} else {
			sshOpts.Username = mc.SSH.RemoteUsername
		}
	}

	err = machine.LocalhostSSHShell(sshOpts.Username, mc.SSH.IdentityPath, mc.Name, mc.SSH.Port, sshOpts.Args)
	return utils.HandleOSExecError(err)
}
