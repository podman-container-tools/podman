//go:build windows

package hypervctl

const VirtualHardDiskType = "Microsoft:Hyper-V:Virtual Hard Disk"

type VirtualHardDiskStorageSettings struct {
	StorageAllocationSettings

	systemSettings *SystemSettings
	driveSettings  *SyntheticDiskDriveSettings
}
