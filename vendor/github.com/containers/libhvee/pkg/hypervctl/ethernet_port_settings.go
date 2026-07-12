//go:build windows

package hypervctl

import (
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

const SyntheticEthernetPortResourceType = "Microsoft:Hyper-V:Synthetic Ethernet Port"
const DefaultSwitchId = "C08CB7B8-9B3C-408E-8E30-5E16A3AEB444"

type SyntheticEthernetPortSettings struct {
	S__PATH                  string
	InstanceID               string
	Caption                  string // = "Virtual Ethernet Port Default Settings"
	Description              string // = "Describes the default settings for the virtual Ethernet port resources."
	ElementName              string
	ResourceType             uint16 // = 10
	OtherResourceType        string
	ResourceSubType          string // = "Microsoft:Hyper-V:Synthetic Ethernet Port"
	PoolID                   string
	ConsumerVisibility       uint16 // = 3
	HostResource             []string
	AllocationUnits          string // = "count"
	VirtualQuantity          uint64 // = 1
	Reservation              uint64 // = 1
	Limit                    uint64 // = 1
	Weight                   uint32 // = 0
	AutomaticAllocation      bool   // = True
	AutomaticDeallocation    bool   // = True
	Parent                   string
	Connection               []string
	Address                  string
	MappingBehavior          uint16
	AddressOnParent          string
	VirtualQuantityUnits     string // = "count"
	DesiredVLANEndpointMode  uint16
	OtherEndpointMode        string
	VirtualSystemIdentifiers []string
	DeviceNamingEnabled      bool // = FALSE
	AllowPacketDirect        bool // = FALSE
	StaticMacAddress         bool // = False
	ClusterMonitored         bool // = TRUE

	systemSettings *SystemSettings
}

func (p *SyntheticEthernetPortSettings) Path() string {
	return p.S__PATH
}

func (p *SyntheticEthernetPortSettings) DefineEthernetPortConnection(switchName string) (*EthernetPortAllocationSettings, error) {
	const wqlFormat = "select * from Msvm_VirtualEthernetSwitch where %s = '%s'"

	var wqlProperty, wqlValue string
	if len(switchName) > 0 {
		wqlProperty = "ElementName"
		wqlValue = switchName
	} else {
		wqlProperty = "Name"
		wqlValue = DefaultSwitchId
	}

	wql := fmt.Sprintf(wqlFormat, wqlProperty, wqlValue)

	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return nil, err
	}
	defer service.Close()

	switchInst, err := service.FindFirstInstance(wql)
	if err != nil {
		return nil, err
	}
	defer switchInst.Close()
	switchPath, err := switchInst.Path()
	if err != nil {
		return nil, err
	}

	connectSettings, err := fetchEthernetPortAllocationSettings()
	if err != nil {
		return nil, err
	}

	connectSettings.Parent = p.Path()
	connectSettings.HostResource = append(connectSettings.HostResource, switchPath)

	resource, err := creatEthernetPortAllocationSettings(connectSettings)
	if err != nil {
		return nil, err
	}

	path, err := addResource(service, p.systemSettings.Path(), resource)
	if err != nil {
		return nil, err
	}

	if err := service.GetObjectAsObject(path, connectSettings); err != nil {
		return nil, err
	}

	return connectSettings, nil
}
