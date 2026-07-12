//go:build windows

package hypervctl

const SyntheticDvdDriveType = "Microsoft:Hyper-V:Synthetic DVD Drive"

type SyntheticDvdDriveSettings struct {
	ResourceSettings
	systemSettings     *SystemSettings
	controllerSettings *ScsiControllerSettings
}

func (d *SyntheticDvdDriveSettings) DefineVirtualDvdDisk(imageFile string) (*VirtualDvdDiskStorageSettings, error) {
	vdvd := &VirtualDvdDiskStorageSettings{}

	if err := createDiskResourceInternal(d.systemSettings.Path(), d.Path(), imageFile, vdvd, VirtualDvdDiskType, nil); err != nil {
		return nil, err
	}

	vdvd.driveSettings = d
	vdvd.systemSettings = d.systemSettings
	return vdvd, nil
}
