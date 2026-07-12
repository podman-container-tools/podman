//go:build windows

package hypervctl

import "time"

type VirtualHardDiskSettings struct {
	S__PATH                    string
	InstanceID                 string
	Caption                    string // = "Virtual Hard Disk Setting Data"
	Description                string // = "Setting Data for a Virtual Hard Disk"
	ElementName                string
	Type                       uint16
	Format                     uint16
	Path                       string
	ParentPath                 string
	ParentTimestamp            time.Time
	ParentIdentifier           string
	MaxInternalSize            uint64
	BlockSize                  uint32
	LogicalSectorSize          uint32
	PhysicalSectorSize         uint32
	VirtualDiskId              string
	DataAlignment              uint64
	PmemAddressAbstractionType uint16
	IsPmemCompatible           bool
}
