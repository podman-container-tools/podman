//go:build !windows && !darwin

package provider

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/qemu"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func Get() (vmconfigs.VMProvider, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, err
	}
	provider := cfg.Machine.Provider
	if providerOverride, found := os.LookupEnv("CONTAINERS_MACHINE_PROVIDER"); found {
		provider = providerOverride
	}
	resolvedVMType, err := define.ParseVMType(provider, define.QemuVirt)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("Using Podman machine with `%s` virtualization provider", resolvedVMType.String())
	return GetByVMType(resolvedVMType)
}

func GetAll() []vmconfigs.VMProvider {
	return []vmconfigs.VMProvider{new(qemu.QEMUStubber)}
}

// GetByVMType takes a VMType (presumably from ParseVMType) and returns the correlating
// VMProvider
func GetByVMType(resolvedVMType define.VMType) (vmconfigs.VMProvider, error) {
	switch resolvedVMType {
	case define.QemuVirt:
		return qemu.NewStubber()
	default:
	}
	return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.QemuVirt}
}

func IsInstalled(provider define.VMType) (bool, error) {
	switch provider {
	case define.QemuVirt:
		cfg, err := config.Default()
		if err != nil {
			return false, err
		}
		if cfg == nil {
			return false, fmt.Errorf("error fetching getting default config")
		}
		_, err = cfg.FindHelperBinary(qemu.QemuCommand, true)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, nil
	}
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	// there are no permissions required for QEMU
	return provider == define.QemuVirt
}
