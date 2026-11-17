//go:build amd64 || arm64

package machine

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/machine"
	machineDefine "go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	provider2 "go.podman.io/podman/v6/pkg/machine/provider"
	"go.podman.io/podman/v6/pkg/machine/shim"
	"gopkg.in/yaml.v3"
)

var infoDescription = `Display information pertaining to the machine host.`

var infoCmd = &cobra.Command{
	Use:               "info [options]",
	Short:             "Display machine host info",
	Long:              infoDescription,
	PersistentPreRunE: machinePreRunE,
	RunE:              info,
	Args:              validate.NoArgs,
	ValidArgsFunction: completion.AutocompleteNone,
	Example:           `podman machine info`,
}

var inFormat string

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: infoCmd,
		Parent:  machineCmd,
	})

	flags := infoCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&inFormat, formatFlagName, "f", "", "Change the output format to JSON or a Go template")
	_ = infoCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.MachineInfo{}))
}

func info(cmd *cobra.Command, _ []string) error {
	info := entities.MachineInfo{}
	version, err := define.GetVersion()
	if err != nil {
		return fmt.Errorf("getting version info %w", err)
	}
	info.Version = version

	host, err := hostInfo()
	if err != nil {
		return err
	}
	info.Host = host

	switch {
	case report.IsJSON(inFormat):
		b, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case cmd.Flags().Changed("format"):
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		// Use OriginUnknown so it does not add an extra range since it
		// will only be called for a single element and not a slice.
		rpt, err = rpt.Parse(report.OriginUnknown, inFormat)
		if err != nil {
			return err
		}
		return rpt.Execute(info)
	default:
		b, err := yaml.Marshal(info)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}

func hostInfo() (*entities.MachineHostInfo, error) {
	host := entities.MachineHostInfo{}

	host.Arch = runtime.GOARCH
	host.OS = runtime.GOOS

	// Enumerate machines across all providers (like "podman machine ls") so the
	// reported details reflect the machine actually in use rather than only the
	// system default provider.
	listResponse, err := shim.List(provider2.GetAll(), machine.ListOptions{})
	if err != nil {
		return nil, err
	}

	host.NumberOfMachines = len(listResponse)

	defaultCon := ""
	con, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetConnection("", true)
	if err == nil {
		// ignore the error here we only want to know if we have a default connection to show it in list
		defaultCon = con.Name
	}

	// Default state of machine is stopped
	host.MachineState = "Stopped"
	// vmType of the machine whose details (VMType, dirs) we report.
	var reportVMType, defaultVMType string
	for _, vm := range listResponse {
		// Set default machine if found
		if vm.Name == defaultCon {
			host.DefaultMachine = vm.Name
			defaultVMType = vm.VMType
		}
		// If machine is running or starting, it is automatically the current machine
		if vm.Running {
			host.CurrentMachine = vm.Name
			host.MachineState = "Running"
			reportVMType = vm.VMType
		} else if vm.Starting {
			host.CurrentMachine = vm.Name
			host.MachineState = "Starting"
			reportVMType = vm.VMType
		}
	}
	// If no machines are starting or running, set current machine to default machine
	// If no default machines are found, do not report a default machine or a state
	if host.CurrentMachine == "" {
		if host.DefaultMachine == "" {
			host.MachineState = ""
		} else {
			host.CurrentMachine = host.DefaultMachine
			reportVMType = defaultVMType
		}
	}

	// Fall back to the system default provider when no machine exists.
	if reportVMType == "" {
		reportVMType = machineProvider.VMType().String()
	}
	host.VMType = reportVMType

	vmType, err := machineDefine.ParseVMType(reportVMType, machineProvider.VMType())
	if err != nil {
		return nil, err
	}
	dirs, err := env.GetMachineDirs(vmType)
	if err != nil {
		return nil, err
	}
	host.MachineImageDir = dirs.DataDir.GetPath()
	host.MachineConfigDir = dirs.ConfigDir.GetPath()

	eventsDir, err := eventSockDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get events dir: %w", err)
	}
	host.EventsDir = eventsDir

	return &host, nil
}
