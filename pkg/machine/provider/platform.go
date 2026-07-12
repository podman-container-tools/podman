package provider

import (
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func InstalledProviders() ([]define.VMType, error) {
	installedTypes := []define.VMType{}
	providers := GetAll()
	for _, p := range providers {
		installed, err := IsInstalled(p.VMType())
		if err != nil {
			return nil, err
		}
		if installed {
			installedTypes = append(installedTypes, p.VMType())
		}
	}
	return installedTypes, nil
}

// GetAllMachinesAndRootfulness collects all podman machine configs and returns
// a map in the format: { machineName: isRootful }
func GetAllMachinesAndRootfulness() (map[string]bool, error) {
	providers := GetAll()
	machines := map[string]bool{}
	for _, provider := range providers {
		dirs, err := env.GetMachineDirs(provider.VMType())
		if err != nil {
			return nil, err
		}
		providerMachines, err := vmconfigs.LoadMachinesInDir(dirs)
		if err != nil {
			return nil, err
		}

		for n, m := range providerMachines {
			machines[n] = m.HostUser.Rootful
		}
	}

	return machines, nil
}
