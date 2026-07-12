//go:build windows

package hypervctl

import (
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

type ScsiControllerSettings struct {
	ResourceSettings
	systemSettings *SystemSettings
}

type driveAssociation interface {
	setParent(parent string)
	setAddressOnParent(address string)
}

func (c *ScsiControllerSettings) AddSyntheticDiskDrive(slot uint) (*SyntheticDiskDriveSettings, error) {
	drive := &SyntheticDiskDriveSettings{}
	if err := c.createSyntheticDriveInternal(slot, drive, SyntheticDiskDriveType); err != nil {
		return nil, err
	}
	drive.systemSettings = c.systemSettings
	drive.controllerSettings = c
	return drive, nil
}

func (c *ScsiControllerSettings) AddSyntheticDvdDrive(slot uint) (*SyntheticDvdDriveSettings, error) {
	drive := &SyntheticDvdDriveSettings{}
	if err := c.createSyntheticDriveInternal(slot, drive, SyntheticDvdDriveType); err != nil {
		return nil, err
	}
	drive.systemSettings = c.systemSettings
	drive.controllerSettings = c
	return drive, nil
}

func (c *ScsiControllerSettings) createSyntheticDriveInternal(slot uint, settings driveAssociation, resourceType string) error {
	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return err
	}
	defer service.Close()

	if err = populateDefaults(resourceType, settings); err != nil {
		return err
	}

	settings.setParent(c.Path())
	settings.setAddressOnParent(fmt.Sprintf("%d", slot))

	driveResource, err := createResourceSettingGeneric(settings, resourceType)
	if err != nil {
		return err
	}

	path, err := addResource(service, c.systemSettings.Path(), driveResource)
	if err != nil {
		return err
	}

	err = service.GetObjectAsObject(path, settings)
	return err
}
