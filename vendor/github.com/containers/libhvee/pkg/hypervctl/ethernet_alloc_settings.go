//go:build windows

package hypervctl

const EthernetPortAllocationResourceType = "Microsoft:Hyper-V:Ethernet Connection"

type EthernetPortAllocationSettings struct {
	InstanceID              string // = "Microsoft:GUID\DeviceSpecificData"
	Caption                 string // = "Ethernet Switch Port Settings"
	Description             string // = "Ethernet Switch Port Settings"
	ElementName             string
	ResourceType            uint16 // = 33
	OtherResourceType       string
	ResourceSubType         string
	PoolID                  string
	ConsumerVisibility      uint16 // = 3
	HostResource            []string
	AllocationUnits         string
	VirtualQuantity         uint64
	Reservation             uint64
	Limit                   uint64
	Weight                  uint32 // = 0
	AutomaticAllocation     bool
	AutomaticDeallocation   bool
	Parent                  string
	Connection              []string
	Address                 string
	MappingBehavior         uint16
	AddressOnParent         string
	VirtualQuantityUnits    string // = "count"
	DesiredVLANEndpointMode uint16
	OtherEndpointMode       string
	EnabledState            uint16
	LastKnownSwitchName     string
	RequiredFeatures        []string
	RequiredFeatureHints    []string
	TestReplicaPoolID       string
	TestReplicaSwitchName   string
	CompartmentGuid         string
}

func fetchEthernetPortAllocationSettings() (*EthernetPortAllocationSettings, error) {
	settings := &EthernetPortAllocationSettings{}
	return settings, populateDefaults(EthernetPortAllocationResourceType, settings)
}

func creatEthernetPortAllocationSettings(settings *EthernetPortAllocationSettings) (string, error) {
	return createResourceSettingGeneric(settings, EthernetPortAllocationResourceType)
}
