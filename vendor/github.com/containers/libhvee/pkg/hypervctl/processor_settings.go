//go:build windows

package hypervctl

import "fmt"

const ProcessorResourceType = "Microsoft:Hyper-V:Processor"

/*
AllocationUnits                : percent / 1000
AllowACountMCount              : True
AutomaticAllocation            : True
AutomaticDeallocation          : True
Caption                        : Processor
Connection                     :
ConsumerVisibility             : 3
CpuGroupId                     : 00000000-0000-0000-0000-000000000000
Description                    : Settings for Microsoft Virtual Processor.
DisableSpeculationControls     : False
ElementName                    : Processor
EnableHostResourceProtection   : False
EnableLegacyApicMode           : False
EnablePageShattering           : 0
EnablePerfmonIpt               : False
EnablePerfmonLbr               : False
EnablePerfmonPebs              : False
EnablePerfmonPmu               : False
ExposeVirtualizationExtensions : False
HideHypervisorPresent          : False
HostResource                   :
HwThreadsPerCore               : 0
InstanceID                     : Microsoft:B5314955-3924-42BA-ABF9-793993D340A0\b637f346-6a0e-4dec-af52-bd70cb80a21d\0
Limit                          : 100000
LimitCPUID                     : False
LimitProcessorFeatures         : False
MappingBehavior                :
MaxNumaNodesPerSocket          : 1
MaxProcessorsPerNumaNode       : 4
OtherResourceType              :
Parent                         :
PoolID                         :
Reservation                    : 0
ResourceSubType                : Microsoft:Hyper-V:Processor
ResourceType                   : 3
VirtualQuantity                : 2
VirtualQuantityUnits           : count
Weight                         : 100
*/

type ProcessorSettings struct {
	S__PATH                        string
	InstanceID                     string
	Caption                        string // = "Processor"
	Description                    string // = "A logical processor of the hypervisor running on the host computer system."
	ElementName                    string
	ResourceType                   uint16 // = 3
	OtherResourceType              string
	ResourceSubType                string // = "Microsoft:Hyper-V:Processor"
	PoolID                         string
	ConsumerVisibility             uint16
	HostResource                   []string
	AllocationUnits                string // = "percent / 1000"
	VirtualQuantity                uint64 // = "count"
	Reservation                    uint64 // = 0
	Limit                          uint64 // = 100000
	Weight                         uint32 // = 100
	AutomaticAllocation            bool   // = True
	AutomaticDeallocation          bool   // = True
	Parent                         string
	Connection                     []string
	Address                        string
	MappingBehavior                uint16
	AddressOnParent                string
	VirtualQuantityUnits           string // = "count"
	LimitCPUID                     bool
	HwThreadsPerCore               uint64
	LimitProcessorFeatures         bool
	MaxProcessorsPerNumaNode       uint64
	MaxNumaNodesPerSocket          uint64
	EnableHostResourceProtection   bool
	CpuGroupId                     string
	HideHypervisorPresent          bool
	ExposeVirtualizationExtensions bool
}

func fetchDefaultProcessorSettings() (*ProcessorSettings, error) {
	settings := &ProcessorSettings{}
	return settings, populateDefaults(ProcessorResourceType, settings)
}

func createProcessorSettings(settings *ProcessorSettings) (string, error) {
	str, err := createResourceSettingGeneric(settings, ProcessorResourceType)
	if err != nil {
		err = fmt.Errorf("could not create processor settings: %w", err)
	}
	return str, err
}
