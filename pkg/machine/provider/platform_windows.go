package provider

import (
	"fmt"
	"os"

	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/hyperv"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/machine/wsl"
	"go.podman.io/podman/v6/pkg/machine/wsl/wutil"
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
	resolvedVMType, err := define.ParseVMType(provider, define.WSLVirt)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Using Podman machine with `%s` virtualization provider", resolvedVMType.String())
	return GetByVMType(resolvedVMType)
}

// GetByVMType takes a VMType (presumably from ParseVMType) and returns the correlating
// VMProvider
func GetByVMType(resolvedVMType define.VMType) (vmconfigs.VMProvider, error) {
	switch resolvedVMType {
	case define.WSLVirt:
		return new(wsl.WSLStubber), nil
	case define.HyperVVirt:
		// Permission checks for Hyper-V are handled at the stubber method level
		// rather than here, because `init` needs to proceed even when the user is
		// not yet in the Hyper-V admin group (it will add them to the group during CreateVM).
		return new(hyperv.HyperVStubber), nil
	default:
	}
	return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
}

func GetAll() []vmconfigs.VMProvider {
	return []vmconfigs.VMProvider{
		new(wsl.WSLStubber),
		new(hyperv.HyperVStubber),
	}
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.HyperVVirt, define.WSLVirt}
}

func IsInstalled(provider define.VMType) (bool, error) {
	switch provider {
	case define.WSLVirt:
		return wutil.IsWSLInstalled(), nil
	case define.HyperVVirt:
		service, err := hypervctl.NewLocalHyperVService()
		if err == nil {
			return true, nil
		}
		if service != nil {
			defer service.Close()
		}
		return false, nil
	default:
		return false, nil
	}
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	switch provider {
	case define.QemuVirt:
		fallthrough
	case define.AppleHvVirt:
		return false
	case define.HyperVVirt:
		err := hyperv.VerifyHyperVPermissions()
		if err != nil {
			logrus.Warn(err)
		}
		return err == nil
	}

	return true
}
