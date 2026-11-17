//go:build windows

package hypervctl

type StorageAllocationSettings struct {
	S__PATH                         string
	InstanceID                      string
	Caption                         string // = "Hard Disk Image Default Settings"
	Description                     string // = "Describes the default settings for the hard disk image resources"
	ElementName                     string
	ResourceType                    uint16
	OtherResourceType               string
	ResourceSubType                 string
	PoolID                          string
	ConsumerVisibility              uint16
	HostResource                    []string
	AllocationUnits                 string
	VirtualQuantity                 uint64
	Limit                           uint64 // = 1
	Weight                          uint32
	StorageQoSPolicyID              string
	AutomaticAllocation             bool
	AutomaticDeallocation           bool
	Parent                          string
	Connection                      []string
	Address                         string
	MappingBehavior                 uint16
	AddressOnParent                 string
	VirtualResourceBlockSize        uint64
	VirtualQuantityUnits            string // = "count(fixed size block)"
	Access                          uint16
	HostResourceBlockSize           uint64
	Reservation                     uint64
	HostExtentStartingAddress       uint64
	HostExtentName                  string
	HostExtentNameFormat            uint16
	OtherHostExtentNameFormat       string
	HostExtentNameNamespace         uint16
	OtherHostExtentNameNamespace    string
	IOPSLimit                       uint64
	IOPSReservation                 uint64
	IOPSAllocationUnits             string
	PersistentReservationsSupported bool
	CachingMode                     uint16
	SnapshotId                      string // = ""
	IgnoreFlushes                   bool
	WriteHardeningMethod            uint16
}

func (s *StorageAllocationSettings) setParent(parent string) {
	s.Parent = parent
}

func (s *StorageAllocationSettings) setHostResource(resource []string) {
	s.HostResource = resource
}

func (s *StorageAllocationSettings) Path() string {
	return s.S__PATH
}
