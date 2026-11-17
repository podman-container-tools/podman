//go:build amd64 || arm64

package machine

import (
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/shim"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] [MACHINE...]",
		Short:             "Inspect an existing machine",
		Long:              "Provide details on a managed virtual machine",
		PersistentPreRunE: machinePreRunE,
		RunE:              inspect,
		Example:           `podman machine inspect myvm`,
		ValidArgsFunction: AutocompleteMachine,
	}
	inspectFlag = inspectFlagType{}
)

type inspectFlagType struct {
	format string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  machineCmd,
	})

	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&inspectFlag.format, formatFlagName, "", "Format volume output using JSON or a Go template")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&machine.InspectInfo{}))
}

func inspect(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if len(args) < 1 {
		args = append(args, defaultMachineName)
	}

	vms := make([]machine.InspectInfo, 0, len(args))
	for _, name := range args {
		mc, vmProvider, err := shim.VMExists(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		dirs, err := env.GetMachineDirs(vmProvider.VMType())
		if err != nil {
			return err
		}

		state, err := vmProvider.State(mc, false)
		if err != nil {
			return err
		}

		podmanSocket, podmanPipe, err := mc.ConnectionInfo(vmProvider.VMType())
		if err != nil {
			return err
		}

		rosetta, err := vmProvider.GetRosetta(mc)
		if err != nil {
			return err
		}

		ii := machine.InspectInfo{
			ConfigDir: *dirs.ConfigDir,
			ConnectionInfo: machine.ConnectionConfig{
				PodmanSocket: podmanSocket,
				PodmanPipe:   podmanPipe,
			},
			Created:            mc.Created,
			LastUp:             mc.LastUp,
			Name:               mc.Name,
			Resources:          mc.Resources,
			SSHConfig:          mc.SSH,
			State:              state,
			UserModeNetworking: vmProvider.UserModeNetworkEnabled(mc),
			Rootful:            mc.HostUser.Rootful,
			Rosetta:            rosetta,
		}

		vms = append(vms, ii)
	}

	switch {
	case cmd.Flag("format").Changed:
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		rpt, err := rpt.Parse(report.OriginUser, inspectFlag.format)
		if err != nil {
			return err
		}

		if err := rpt.Execute(vms); err != nil {
			errs = append(errs, err)
		}
	default:
		if err := printJSON(vms); err != nil {
			errs = append(errs, err)
		}
	}
	return errs.PrintErrors()
}

func printJSON(data []machine.InspectInfo) error {
	enc := json.NewEncoder(os.Stdout)
	// by default, json marshallers will force utf=8 from
	// a string. this breaks healthchecks that use <,>, &&.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "     ")
	return enc.Encode(data)
}
