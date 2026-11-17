//go:build windows

package hypervctl

import (
	"github.com/containers/libhvee/pkg/wmiext"
)

const SyntheticDiskDriveType = "Microsoft:Hyper-V:Synthetic Disk Drive"

type SyntheticDiskDriveSettings struct {
	ResourceSettings
	systemSettings     *SystemSettings
	controllerSettings *ScsiControllerSettings
}

type diskAssociation interface {
	setParent(parent string)
	setHostResource(resource []string)
	Path() string
}

func (d *SyntheticDiskDriveSettings) DefineVirtualHardDisk(vhdxFile string, beforeAdd func(*VirtualHardDiskStorageSettings)) (*VirtualHardDiskStorageSettings, error) {
	vhd := &VirtualHardDiskStorageSettings{}

	var cb func()
	if beforeAdd != nil {
		cb = func() {
			beforeAdd(vhd)
		}
	}

	if err := createDiskResourceInternal(d.systemSettings.Path(), d.Path(), vhdxFile, vhd, VirtualHardDiskType, cb); err != nil {
		return nil, err
	}

	vhd.driveSettings = d
	vhd.systemSettings = d.systemSettings
	return vhd, nil
}

func createDiskResourceInternal(systemPath string, drivePath string, file string, settings diskAssociation, resourceType string, cb func()) error {
	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return err
	}
	defer service.Close()

	if err = populateDefaults(resourceType, settings); err != nil {
		return err
	}

	settings.setHostResource([]string{file})
	settings.setParent(drivePath)
	if cb != nil {
		cb()
	}

	diskResource, err := createResourceSettingGeneric(settings, resourceType)
	if err != nil {
		return err
	}

	path, err := addResource(service, systemPath, diskResource)
	if err != nil {
		return err
	}

	return service.GetObjectAsObject(path, settings)
}
