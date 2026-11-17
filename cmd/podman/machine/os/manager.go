//go:build amd64 || arm64

package os

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	machineconfig "go.podman.io/common/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	pkgOS "go.podman.io/podman/v6/pkg/machine/os"
	"go.podman.io/podman/v6/pkg/machine/shim"
)

type ManagerOpts struct {
	VMName  string
	CLIArgs []string
	Restart bool
}

// NewOSManager creates a new OSManager depending on the mode of the call
func NewOSManager(opts ManagerOpts) (pkgOS.Manager, error) {
	// If a VM name is specified, then we know that we are not inside a
	// Podman VM, but rather outside of it.
	if machineconfig.IsPodmanMachine() && opts.VMName == "" {
		return guestOSManager()
	}

	// Set to the default name if no VM was provided
	if opts.VMName == "" {
		opts.VMName = define.DefaultMachineName
	}

	mc, vmProvider, err := shim.VMExists(opts.VMName)
	if err != nil {
		return nil, err
	}

	if vmProvider.VMType() == define.WSLVirt {
		return nil, errors.New("this command is not supported for WSL")
	}
	return &pkgOS.MachineOS{
		VM:       mc,
		Provider: vmProvider,
		Args:     opts.CLIArgs,
		VMName:   opts.VMName,
		Restart:  opts.Restart,
	}, nil
}

// guestOSManager returns an OSmanager for inside-VM operations
func guestOSManager() (pkgOS.Manager, error) {
	dist, err := GetDistribution()
	if err != nil {
		return nil, fmt.Errorf("failed to read os-release file: %w", err)
	}
	switch {
	case dist.Name == "fedora" && dist.Variant == "podman-machine-os":
		return &pkgOS.OSTree{}, nil
	default:
		return nil, fmt.Errorf("unsupported OS/Variant: %s/%s", dist.Name, dist.Variant)
	}
}

type Distribution struct {
	Name    string
	Variant string
}

// GetDistribution checks the OS distribution
func GetDistribution() (Distribution, error) {
	dist := Distribution{}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return dist, err
	}
	defer f.Close()

	l := bufio.NewScanner(f)
	for l.Scan() {
		if after, ok := strings.CutPrefix(l.Text(), "ID="); ok {
			dist.Name = after
		}
		if after, ok := strings.CutPrefix(l.Text(), "VARIANT_ID="); ok {
			dist.Variant = strings.Trim(after, "\"")
		}
	}
	if err := l.Err(); err != nil {
		return dist, err
	}
	return dist, nil
}
