//go:build windows

package hypervctl

const VirtualDvdDiskType = "Microsoft:Hyper-V:Virtual CD/DVD Disk"

type VirtualDvdDiskStorageSettings struct {
	StorageAllocationSettings

	systemSettings *SystemSettings
	driveSettings  *SyntheticDvdDriveSettings
}
