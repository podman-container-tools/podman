package config

import (
	"fmt"
	"strings"

	"github.com/crc-org/vfkit/pkg/util"
)

// Bootloader is the base interface for all bootloader classes. It specifies how to
// boot the virtual machine. It is mandatory to set a Bootloader or the virtual
// machine won't start.
type Bootloader interface {
	FromOptions(options []option) error
	ToCmdLine() ([]string, error)
}

// LinuxBootloader determines which kernel/initrd/kernel args to use when starting
// the virtual machine.
type LinuxBootloader struct {
	VmlinuzPath   string `json:"vmlinuzPath"`
	KernelCmdLine string `json:"kernelCmdLine"`
	InitrdPath    string `json:"initrdPath"`
}

// EFIBootloader allows to set a few options related to EFI variable storage
type EFIBootloader struct {
	EFIVariableStorePath string `json:"efiVariableStorePath"`
	// TODO: virtualization framework allow both create and overwrite
	CreateVariableStore bool `json:"createVariableStore"`
}

// MacOSBootloader provides necessary objects for booting macOS guests
type MacOSBootloader struct {
	MachineIdentifierPath string `json:"machineIdentifierPath"`
	HardwareModelPath     string `json:"hardwareModelPath"`
	AuxImagePath          string `json:"auxImagePath"`
}

// NewLinuxBootloader creates a new bootloader to start a VM with the file at
// vmlinuzPath as the kernel, kernelCmdLine as the kernel command line, and the
// file at initrdPath as the initrd. On ARM64, the kernel must be uncompressed
// otherwise the VM will fail to boot.
func NewLinuxBootloader(vmlinuzPath, kernelCmdLine, initrdPath string) *LinuxBootloader {
	return &LinuxBootloader{
		VmlinuzPath:   vmlinuzPath,
		KernelCmdLine: kernelCmdLine,
		InitrdPath:    initrdPath,
	}
}

func (bootloader *LinuxBootloader) FromOptions(options []option) error {
	for _, option := range options {
		switch option.key {
		case "kernel":
			bootloader.VmlinuzPath = option.value
		case "cmdline":
			bootloader.KernelCmdLine = util.TrimQuotes(option.value)
		case "initrd":
			bootloader.InitrdPath = option.value
		default:
			return fmt.Errorf("unknown option for Linux bootloaders: %s", option.key)
		}
	}
	return nil
}

func (bootloader *LinuxBootloader) ToCmdLine() ([]string, error) {
	args := []string{}
	if bootloader.VmlinuzPath == "" {
		return nil, fmt.Errorf("missing kernel path")
	}
	args = append(args, "--kernel", bootloader.VmlinuzPath)

	if bootloader.InitrdPath == "" {
		return nil, fmt.Errorf("missing initrd path")
	}
	args = append(args, "--initrd", bootloader.InitrdPath)

	if bootloader.KernelCmdLine == "" {
		return nil, fmt.Errorf("missing kernel command line")
	}
	args = append(args, "--kernel-cmdline", bootloader.KernelCmdLine)

	return args, nil
}

// NewEFIBootloader creates a new bootloader to start a VM using EFI
// efiVariableStorePath is the path to a file for EFI storage
// create is a boolean indicating if the file for the store should be created or not
func NewEFIBootloader(efiVariableStorePath string, createVariableStore bool) *EFIBootloader {
	return &EFIBootloader{
		EFIVariableStorePath: efiVariableStorePath,
		CreateVariableStore:  createVariableStore,
	}
}

func (bootloader *EFIBootloader) FromOptions(options []option) error {
	for _, option := range options {
		switch option.key {
		case "variable-store":
			bootloader.EFIVariableStorePath = option.value
		case "create":
			if option.value != "" {
				return fmt.Errorf("unexpected value for EFI bootloader 'create' option: %s", option.value)
			}
			bootloader.CreateVariableStore = true
		default:
			return fmt.Errorf("unknown option for EFI bootloaders: %s", option.key)
		}
	}
	return nil
}

func (bootloader *EFIBootloader) ToCmdLine() ([]string, error) {
	if bootloader.EFIVariableStorePath == "" {
		return nil, fmt.Errorf("missing EFI store path")
	}

	builder := strings.Builder{}
	builder.WriteString("efi")
	fmt.Fprintf(&builder, ",variable-store=%s", bootloader.EFIVariableStorePath)
	if bootloader.CreateVariableStore {
		builder.WriteString(",create")
	}

	return []string{"--bootloader", builder.String()}, nil
}

func (bootloader *MacOSBootloader) FromOptions(options []option) error {
	for _, option := range options {
		switch option.key {
		case "machineIdentifierPath":
			bootloader.MachineIdentifierPath = option.value
		case "hardwareModelPath":
			bootloader.HardwareModelPath = option.value
		case "auxImagePath":
			bootloader.AuxImagePath = option.value
		default:
			return fmt.Errorf("unknown option for macOS bootloaders: %s", option.key)
		}
	}
	return nil
}

func (bootloader *MacOSBootloader) ToCmdLine() ([]string, error) {
	args := []string{}

	return args, nil
}

func BootloaderFromCmdLine(optsStrv []string) (Bootloader, error) {
	var bootloader Bootloader

	if len(optsStrv) < 1 {
		return nil, fmt.Errorf("empty option list in --bootloader command line argument")
	}
	bootloaderType := optsStrv[0]
	switch bootloaderType {
	case "efi":
		bootloader = &EFIBootloader{}
	case "linux":
		bootloader = &LinuxBootloader{}
	case "macos":
		bootloader = &MacOSBootloader{}
	default:
		return nil, fmt.Errorf("unknown bootloader type: %s", bootloaderType)
	}
	options := strvToOptions(optsStrv[1:])
	if err := bootloader.FromOptions(options); err != nil {
		return nil, err
	}
	return bootloader, nil
}
