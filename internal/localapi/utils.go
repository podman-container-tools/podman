//go:build amd64 || arm64

package localapi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/provider"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/storage/pkg/fileutils"
)

// FindMachineByPort finds a running machine that matches the given connection port.
// It returns the machine configuration and provider, or an error if not found.
func FindMachineByPort(connectionURI string, parsedConnection *url.URL) (*vmconfigs.MachineConfig, vmconfigs.VMProvider, error) {
	for _, machineProvider := range provider.GetAll() {
		logrus.Debugf("Checking provider: %s", machineProvider.VMType())
		dirs, err := env.GetMachineDirs(machineProvider.VMType())
		if err != nil {
			logrus.Debugf("Failed to get machine dirs for provider %s: %v", machineProvider.VMType(), err)
			continue
		}

		machineList, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			logrus.Debugf("Failed to list machines: %v", err)
			continue
		}

		// Now we know that the connection points to a machine and we
		// can find the machine by looking for the one with the
		// matching port.
		connectionPort, err := strconv.Atoi(parsedConnection.Port())
		if err != nil {
			logrus.Debugf("Failed to parse connection port: %v", err)
			continue
		}

		for _, mc := range machineList {
			if connectionPort != mc.SSH.Port {
				continue
			}

			state, err := machineProvider.State(mc, false)
			if err != nil {
				logrus.Debugf("Failed to get machine state for %s: %v", mc.Name, err)
				continue
			}

			if state != define.Running {
				return nil, nil, fmt.Errorf("machine %s is not running but in state %s", mc.Name, state)
			}

			return mc, machineProvider, nil
		}
	}

	return nil, nil, fmt.Errorf("could not find a matching machine for connection %q", connectionURI)
}

// getMachineMountsAndVMType retrieves the mounts and VM type of a machine based on the connection URI and parsed URL.
// It returns a slice of mounts, the VM type, or an error if the machine cannot be found or is not running.
func getMachineMountsAndVMType(connectionURI string, parsedConnection *url.URL) ([]*vmconfigs.Mount, define.VMType, error) {
	mc, machineProvider, err := FindMachineByPort(connectionURI, parsedConnection)
	if err != nil {
		return nil, define.UnknownVirt, err
	}
	return mc.Mounts, machineProvider.VMType(), nil
}

// IsPathAvailableOnMachine checks if a local path is available on the machine through mounted directories.
// If the path is available, it returns a LocalAPIMap with the corresponding remote path.
func IsPathAvailableOnMachine(mounts []*vmconfigs.Mount, vmType define.VMType, localPath string) (*LocalAPIMap, bool) {
	pathABS, err := filepath.Abs(localPath)
	if err != nil {
		logrus.Debugf("Failed to get absolute path for %q: %v", localPath, err)
		return nil, false
	}

	// WSLVirt is a special case where there is no real concept of doing a mount in WSL,
	// WSL by default mounts the drives to /mnt/c, /mnt/d, etc...
	if vmType == define.WSLVirt {
		converted_path, err := specgen.ConvertWinMountPath(pathABS)
		if err != nil {
			logrus.Debugf("Failed to convert Windows mount path: %v", err)
			return nil, false
		}

		return &LocalAPIMap{
			ClientPath: pathABS,
			RemotePath: converted_path,
		}, true
	}

	for _, mount := range mounts {
		mountSource := filepath.Clean(mount.Source)
		logrus.Debugf("looking if %q is in mount %q (with target %q)", pathABS, mountSource, mount.Target)
		relPath, err := filepath.Rel(mountSource, pathABS)
		if err != nil {
			logrus.Debugf("Failed to get relative path: %v", err)
			continue
		} else {
			logrus.Debugf("Yes, filepath.Rel returned %q", relPath)
		}
		// If relPath starts with ".." or is absolute, pathABS is not under mountSource
		if relPath == "." || (!strings.HasPrefix(relPath, "..") && !filepath.IsAbs(relPath)) {
			// Target is a Linux path, we should use path.Join rather than filepath.Join
			converted_path := path.Join(mount.Target, filepath.ToSlash(relPath))
			logrus.Debugf("Converted client path: %q", converted_path)
			return &LocalAPIMap{
				ClientPath: pathABS,
				RemotePath: converted_path,
			}, true
		}
	}
	return nil, false
}

// CheckPathOnRunningMachine is a convenience function that checks if a path is available
// on any currently running machine. It combines machine inspection and path checking.
func CheckPathOnRunningMachine(ctx context.Context, path string) (*LocalAPIMap, bool) {
	if err := fileutils.Exists(path); errors.Is(err, fs.ErrNotExist) {
		logrus.Debugf("Path %q does not exist locally, skipping machine check", path)
		return nil, false
	}

	if machineMode := bindings.GetMachineMode(ctx); !machineMode {
		logrus.Debug("Machine mode is not enabled, skipping machine check")
		return nil, false
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		logrus.Debugf("Failed to get client connection: %v", err)
		return nil, false
	}

	mounts, vmType, err := getMachineMountsAndVMType(conn.URI.String(), conn.URI)
	if err != nil {
		logrus.Debugf("Failed to get machine mounts: %v", err)
		return nil, false
	}

	return IsPathAvailableOnMachine(mounts, vmType, path)
}

// CheckIfImageBuildPathsOnRunningMachine checks if the build context directory and all specified
// Containerfiles are available on the running machine. If they are, it translates their paths
// to the corresponding remote paths and returns them along with a flag indicating success.
func CheckIfImageBuildPathsOnRunningMachine(ctx context.Context, containerFiles []string, options entities.BuildOptions) ([]string, entities.BuildOptions, bool) {
	if machineMode := bindings.GetMachineMode(ctx); !machineMode {
		logrus.Debug("Machine mode is not enabled, skipping machine check")
		return nil, options, false
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		logrus.Debugf("Failed to get client connection: %v", err)
		return nil, options, false
	}

	mounts, vmType, err := getMachineMountsAndVMType(conn.URI.String(), conn.URI)
	if err != nil {
		logrus.Debugf("Failed to get machine mounts: %v", err)
		return nil, options, false
	}

	// Context directory
	if err := fileutils.Lexists(options.ContextDirectory); errors.Is(err, fs.ErrNotExist) {
		logrus.Debugf("Path %q does not exist locally, skipping machine check", options.ContextDirectory)
		return nil, options, false
	}
	mapping, found := IsPathAvailableOnMachine(mounts, vmType, options.ContextDirectory)
	if !found {
		logrus.Debugf("Path %q is not available on the running machine", options.ContextDirectory)
		return nil, options, false
	}
	options.ContextDirectory = mapping.RemotePath

	// Containerfiles
	translatedContainerFiles := []string{}
	for _, containerFile := range containerFiles {
		if strings.HasPrefix(containerFile, "http://") || strings.HasPrefix(containerFile, "https://") {
			translatedContainerFiles = append(translatedContainerFiles, containerFile)
			continue
		}

		// If Containerfile does not exist, assume it is in context directory
		if err := fileutils.Lexists(containerFile); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.Fatalf("Failed to check if containerfile %s exists: %v", containerFile, err)
				return nil, options, false
			}
			continue
		}

		mapping, found := IsPathAvailableOnMachine(mounts, vmType, containerFile)
		if !found {
			logrus.Debugf("Path %q is not available on the running machine", containerFile)
			return nil, options, false
		}
		translatedContainerFiles = append(translatedContainerFiles, mapping.RemotePath)
	}

	// Additional build contexts
	for _, context := range options.AdditionalBuildContexts {
		switch {
		case context.IsImage, context.IsURL:
			continue
		default:
			if err := fileutils.Lexists(context.Value); errors.Is(err, fs.ErrNotExist) {
				logrus.Debugf("Path %q does not exist locally, skipping machine check", context.Value)
				return nil, options, false
			}
			mapping, found := IsPathAvailableOnMachine(mounts, vmType, context.Value)
			if !found {
				logrus.Debugf("Path %q is not available on the running machine", context.Value)
				return nil, options, false
			}
			context.Value = mapping.RemotePath
		}
	}
	return translatedContainerFiles, options, true
}

func getVmProviderType(ctx context.Context) (define.VMType, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		logrus.Debugf("Failed to get client connection: %v", err)
		return define.UnknownVirt, err
	}

	_, vmProvider, err := FindMachineByPort(conn.URI.String(), conn.URI)
	if err != nil {
		logrus.Debugf("Failed to get machine hypervisor type: %v", err)
		return define.UnknownVirt, err
	}

	return vmProvider.VMType(), nil
}

// IsHyperVProvider checks if the current machine provider is Hyper-V.
// It returns true if the provider is Hyper-V, false otherwise, or an error if the check fails.
func IsHyperVProvider(ctx context.Context) (bool, error) {
	providerType, err := getVmProviderType(ctx)
	return providerType == define.HyperVVirt, err
}

// IsWSLProvider checks if the current machine provider is WSL.
// It returns true if the provider is WSL, false otherwise, or an error if the check fails.
func IsWSLProvider(ctx context.Context) (bool, error) {
	providerType, err := getVmProviderType(ctx)
	return providerType == define.WSLVirt, err
}

// ValidatePathForLocalAPI checks if the provided path satisfies requirements for local API usage.
// It returns an error if the path is not absolute or does not exist on the filesystem.
func ValidatePathForLocalAPI(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: %q", ErrPathNotAbsolute, path)
	}

	if err := fileutils.Exists(path); err != nil {
		return err
	}
	return nil
}
