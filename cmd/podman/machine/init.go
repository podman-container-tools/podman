//go:build amd64 || arm64

package machine

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/strongunits"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/registry"
	ldefine "go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/provider"
	"go.podman.io/podman/v6/pkg/machine/shim"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

var (
	initCmd = &cobra.Command{
		Use:               "init [options] [NAME]",
		Short:             "Initialize a virtual machine",
		Long:              "Initialize a virtual machine",
		PersistentPreRunE: machinePreRunE,
		RunE:              initMachine,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine init podman-machine-default`,
		ValidArgsFunction: completion.AutocompleteNone,
	}

	initOpts           = define.InitOptions{}
	initOptionalFlags  = InitOptionalFlags{}
	defaultMachineName = define.DefaultMachineName
	now                bool
	providerOverride   string
)

// Flags which have a meaning when unspecified that differs from the flag default
type InitOptionalFlags struct {
	UserModeNetworking bool
	tlsVerify          bool
}

// maxMachineNameSize is set to thirty to limit huge machine names primarily
// because macOS has a much smaller file size limit.
const maxMachineNameSize = 30

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: initCmd,
		Parent:  machineCmd,
	})
	flags := initCmd.Flags()
	cfg := registry.PodmanConfig()

	cpusFlagName := "cpus"
	flags.Uint64Var(
		&initOpts.CPUS,
		cpusFlagName, cfg.ContainersConfDefaultsRO.Machine.CPUs,
		"Number of CPUs",
	)
	_ = initCmd.RegisterFlagCompletionFunc(cpusFlagName, completion.AutocompleteNone)

	runPlaybookFlagName := "playbook"
	flags.StringVar(&initOpts.PlaybookPath, runPlaybookFlagName, "", "Run an Ansible playbook after first boot")
	_ = initCmd.RegisterFlagCompletionFunc(runPlaybookFlagName, completion.AutocompleteDefault)

	diskSizeFlagName := "disk-size"
	flags.Uint64Var(
		&initOpts.DiskSize,
		diskSizeFlagName, cfg.ContainersConfDefaultsRO.Machine.DiskSize,
		"Disk size in GiB",
	)

	_ = initCmd.RegisterFlagCompletionFunc(diskSizeFlagName, completion.AutocompleteNone)

	memoryFlagName := "memory"
	flags.Uint64VarP(
		&initOpts.Memory,
		memoryFlagName, "m", cfg.ContainersConfDefaultsRO.Machine.Memory,
		"Memory in MiB",
	)
	_ = initCmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)

	swapFlagName := "swap"
	flags.Uint64VarP(
		&initOpts.Swap,
		swapFlagName, "s", 0,
		"Swap in MiB",
	)
	_ = initCmd.RegisterFlagCompletionFunc(swapFlagName, completion.AutocompleteNone)

	flags.BoolVar(
		&now,
		"now", false,
		"Start machine now",
	)
	timezoneFlagName := "timezone"
	defaultTz := cfg.ContainersConfDefaultsRO.TZ()
	if len(defaultTz) < 1 {
		defaultTz = "local"
	}
	flags.StringVar(&initOpts.TimeZone, timezoneFlagName, defaultTz, "Set timezone")
	_ = initCmd.RegisterFlagCompletionFunc(timezoneFlagName, completion.AutocompleteDefault)

	flags.BoolVar(
		&initOpts.ReExec,
		"reexec", false,
		"process was rexeced",
	)
	_ = flags.MarkHidden("reexec")

	UsernameFlagName := "username"
	flags.StringVar(&initOpts.Username, UsernameFlagName, cfg.ContainersConfDefaultsRO.Machine.User, "Username used in image")
	_ = initCmd.RegisterFlagCompletionFunc(UsernameFlagName, completion.AutocompleteDefault)

	ImageFlagName := "image"
	flags.StringVar(&initOpts.Image, ImageFlagName, cfg.ContainersConfDefaultsRO.Machine.Image, "Bootable image for machine")
	_ = initCmd.RegisterFlagCompletionFunc(ImageFlagName, completion.AutocompleteDefault)

	// Deprecate image-path option, use --image instead
	ImagePathFlagName := "image-path"
	flags.StringVar(&initOpts.Image, ImagePathFlagName, cfg.ContainersConfDefaultsRO.Machine.Image, "Bootable image for machine")
	_ = initCmd.RegisterFlagCompletionFunc(ImagePathFlagName, completion.AutocompleteDefault)
	if err := flags.MarkDeprecated(ImagePathFlagName, "use --image instead"); err != nil {
		logrus.Error("unable to mark image-path flag deprecated")
	}

	VolumeFlagName := "volume"
	flags.StringArrayVarP(&initOpts.Volumes, VolumeFlagName, "v", cfg.ContainersConfDefaultsRO.Machine.Volumes.Get(), "Volumes to mount, source:target")
	_ = initCmd.RegisterFlagCompletionFunc(VolumeFlagName, completion.AutocompleteDefault)

	USBFlagName := "usb"
	flags.StringArrayVarP(&initOpts.USBs, USBFlagName, "", []string{},
		"USB Host passthrough: bus=$1,devnum=$2 or vendor=$1,product=$2")
	_ = initCmd.RegisterFlagCompletionFunc(USBFlagName, completion.AutocompleteDefault)

	VolumeDriverFlagName := "volume-driver"
	flags.String(VolumeDriverFlagName, "", "Optional volume driver")
	_ = initCmd.RegisterFlagCompletionFunc(VolumeDriverFlagName, completion.AutocompleteDefault)
	if err := flags.MarkDeprecated(VolumeDriverFlagName, "will be ignored"); err != nil {
		logrus.Error("unable to mark volume-driver flag deprecated")
	}

	IgnitionPathFlagName := "ignition-path"
	flags.StringVar(&initOpts.IgnitionPath, IgnitionPathFlagName, "", "Path to ignition file")
	_ = initCmd.RegisterFlagCompletionFunc(IgnitionPathFlagName, completion.AutocompleteDefault)

	rootfulFlagName := "rootful"
	flags.BoolVar(&initOpts.Rootful, rootfulFlagName, false, "Whether this machine should prefer rootful container execution")

	userModeNetFlagName := "user-mode-networking"
	flags.BoolVar(&initOptionalFlags.UserModeNetworking, userModeNetFlagName, false,
		"Whether this machine should use user-mode networking, routing traffic through a host user-space process")

	flags.BoolVar(&initOptionalFlags.tlsVerify, "tls-verify", true,
		"Require HTTPS and verify certificates when contacting registries")

	providerFlagName := "provider"
	flags.StringVar(&providerOverride, providerFlagName, "", "Override the default machine provider")
	_ = initCmd.RegisterFlagCompletionFunc(providerFlagName, autocompleteMachineProvider)

	setDefaultConnectionFlagName := "update-connection"
	flags.BoolVarP(&setDefaultSystemConn, setDefaultConnectionFlagName, "u", false, "Set default system connection for this machine")

	importNativeCaFlagName := "import-native-ca"
	flags.BoolVar(&initOpts.ImportNativeCA, importNativeCaFlagName, cfg.ContainersConfDefaultsRO.Machine.ImportNativeCA, "Import the host trusted CA certificates into the machine")
}

func initMachine(cmd *cobra.Command, args []string) error {
	initOpts.Name = defaultMachineName
	if len(args) > 0 {
		if len(args[0]) > maxMachineNameSize {
			return fmt.Errorf("machine name %q must be %d characters or less", args[0], maxMachineNameSize)
		}
		initOpts.Name = args[0]

		if !ldefine.NameRegex.MatchString(initOpts.Name) {
			return fmt.Errorf("invalid name %q: %w", initOpts.Name, ldefine.RegexError)
		}
	}

	// If the provider option was given, we need to override the
	// current provider
	if cmd.Flags().Changed("provider") {
		// var found bool
		indexFunc := func(s vmconfigs.VMProvider) bool {
			return s.VMType().String() == providerOverride
		}
		providers := provider.GetAll()
		indexVal := slices.IndexFunc(providers, indexFunc)
		if indexVal == -1 {
			return fmt.Errorf("unsupported provider %q", providerOverride)
		}
		machineProvider = providers[indexVal]
	}

	// The vmtype names need to be reserved and cannot be used for podman machine names
	if _, err := define.ParseVMType(initOpts.Name, define.UnknownVirt); err == nil {
		return fmt.Errorf("cannot use %q for a machine name", initOpts.Name)
	}

	if !ldefine.NameRegex.MatchString(initOpts.Username) {
		return fmt.Errorf("invalid username %q: %w", initOpts.Username, ldefine.RegexError)
	}

	// Check if machine already exists
	var errNotExists *define.ErrVMDoesNotExist
	_, _, err := shim.VMExists(initOpts.Name)
	// if nil, means we found a vm and need to reject it by name
	if err == nil {
		return &define.ErrVMAlreadyExists{Name: initOpts.Name}
	}
	if !errors.As(err, &errNotExists) {
		return err
	}

	// Check if something on the hypervisor exists with the same name
	exists, err := shim.VMExistsOnHyperVisor(initOpts.Name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%s already exists on hypervisor", initOpts.Name)
	}

	// check if a system connection already exists
	cons, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetAllConnections()
	if err != nil {
		return err
	}
	for _, con := range cons {
		if con.ReadWrite {
			for _, connection := range []string{initOpts.Name, fmt.Sprintf("%s-root", initOpts.Name)} {
				if con.Name == connection {
					return fmt.Errorf("system connection %q already exists. consider a different machine name or remove the connection with `podman system connection rm`", connection)
				}
			}
		}
	}

	for idx, vol := range initOpts.Volumes {
		initOpts.Volumes[idx] = os.ExpandEnv(vol)
	}

	// Process optional flags (flags where unspecified / nil has meaning )
	if cmd.Flags().Changed("user-mode-networking") {
		initOpts.UserModeNetworking = &initOptionalFlags.UserModeNetworking
	}

	if err := checkMaxMemory(strongunits.MiB(initOpts.Memory)); err != nil {
		return err
	}
	if err := checkMaxCPUs(initOpts.CPUS); err != nil {
		return err
	}

	// initOpts.SkipTlsVerify defaults to OptionalBoolUndefined, which means the backend library
	// decides whether to verify TLS. We only explicitly set it if the user specifies the
	// --tls-verify flag on the CLI.
	//
	// The flag value from initOptionalFlags.tlsVerify indicates whether TLS verification is desired.
	// Since we are converting tlsVerify -> SkipTlsVerify, we must invert the bool accordingly.
	if cmd.Flags().Changed("tls-verify") {
		initOpts.SkipTlsVerify = types.NewOptionalBool(!initOptionalFlags.tlsVerify)
	}

	// TODO need to work this back in
	// if finished, err := vm.Init(initOpts); err != nil || !finished {
	// 	// Finished = true,  err  = nil  -  Success! Log a message with further instructions
	// 	// Finished = false, err  = nil  -  The installation is partially complete and podman should
	// 	//                                  exit gracefully with no error and no success message.
	// 	//                                  Examples:
	// 	//                                  - a user has chosen to perform their own reboot
	// 	//                                  - reexec for limited admin operations, returning to parent
	// 	// Finished = *,     err != nil  -  Exit with an error message
	// 	return err
	// }

	err = shim.Init(initOpts, machineProvider)
	if err != nil {
		// ErrRelaunchSucceeded is not a real error: it signals that
		// an elevated child process completed init successfully.
		// Exit gracefully with a success message.
		//
		// This can happen with WSL when installing the WSL features
		// or with HyperV when adding entries to the Registry
		if errors.Is(err, define.ErrRelaunchSucceeded) {
			fmt.Println("Machine init complete")
			if now {
				fmt.Printf("Machine %q started successfully\n", initOpts.Name)
				return nil
			}
			printStartCommand(initOpts.Name)
			return nil
		}
		return err
	}

	newMachineEvent(events.Init, events.Event{Name: initOpts.Name})
	fmt.Println("Machine init complete")

	if now {
		// Pass reexec flag from init to start
		startOpts.ReExec = initOpts.ReExec
		return start(cmd, args)
	}

	printStartCommand(initOpts.Name)
	return err
}

func printStartCommand(machineName string) {
	extra := ""
	if machineName != defaultMachineName {
		extra = " " + machineName
	}
	fmt.Printf("To start your machine run:\n\n\tpodman machine start%s\n\n", extra)
}

// checkMaxMemory gets the total system memory and compares it to the variable.  if the variable
// is larger than the total memory, it returns an error
func checkMaxMemory(newMem strongunits.MiB) error {
	memStat, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	if total := strongunits.B(memStat.Total); strongunits.B(memStat.Total) < newMem.ToBytes() {
		return fmt.Errorf("requested amount of memory (%d MB) greater than total system memory (%d MB)", newMem, strongunits.ToMib(total))
	}
	return nil
}

// checkMaxCPUs compares requested CPUs to the host (runtime.NumCPU).
func checkMaxCPUs(requestedCPUs uint64) error {
	hostCPUs := uint64(runtime.NumCPU())
	if requestedCPUs > hostCPUs {
		return fmt.Errorf("requested number of CPUs (%d) greater than number of host CPUs (%d)", requestedCPUs, hostCPUs)
	}
	return nil
}
