package provider

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/machine/applehv"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/libkrun"
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
	resolvedVMType, err := define.ParseVMType(provider, define.LibKrun)
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
	case define.AppleHvVirt:
		return new(applehv.AppleHVStubber), nil
	case define.LibKrun:
		return new(libkrun.LibKrunStubber), nil
	default:
	}
	return nil, fmt.Errorf("unsupported virtualization provider: `%s`", resolvedVMType.String())
}

func GetAll() []vmconfigs.VMProvider {
	return []vmconfigs.VMProvider{new(libkrun.LibKrunStubber), new(applehv.AppleHVStubber)}
}

// SupportedProviders returns the providers that are supported on the host operating system
func SupportedProviders() []define.VMType {
	return []define.VMType{define.AppleHvVirt, define.LibKrun}
}

func IsInstalled(provider define.VMType) (bool, error) {
	switch provider {
	case define.AppleHvVirt:
		ahv, err := appleHvInstalled()
		if err != nil {
			return false, err
		}
		return ahv, nil
	case define.LibKrun:
		lkr, err := libKrunInstalled()
		if err != nil {
			return false, err
		}
		return lkr, nil
	default:
		return false, nil
	}
}

func appleHvInstalled() (bool, error) {
	var outBuf bytes.Buffer
	// Apple's Virtualization.Framework is only supported on MacOS 11.0+,
	// but to use EFI MacOS 13.0+ is required
	expectedVer, err := semver.Make("13.0.0")
	if err != nil {
		return false, err
	}

	cmd := exec.Command("sw_vers", "--productVersion")
	cmd.Stdout = &outBuf
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("unable to check current macOS version using `sw_vers --productVersion`: %s", err)
	}

	// the output will be in the format of MAJOR.MINOR.PATCH
	output := strings.TrimSuffix(outBuf.String(), "\n")
	currentVer, err := semver.Make(output)
	if err != nil {
		return false, err
	}

	return currentVer.GTE(expectedVer), nil
}

func libKrunInstalled() (bool, error) {
	// need to verify that krunkit, virglrenderer, and libkrun-efi are installed
	cfg, err := config.Default()
	if err != nil {
		return false, err
	}

	_, err = cfg.FindHelperBinary("krunkit", false)
	return err == nil, nil
}

// HasPermsForProvider returns whether the host operating system has the proper permissions to use the given provider
func HasPermsForProvider(provider define.VMType) bool {
	// there are no permissions required for AppleHV or LibKrun
	return provider == define.AppleHvVirt || provider == define.LibKrun
}
