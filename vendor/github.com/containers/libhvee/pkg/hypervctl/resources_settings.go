//go:build windows

package hypervctl

import (
	"errors"
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

type ResourceSettings struct {
	S__PATH                  string
	InstanceID               string // = "Microsoft:GUID\DeviceSpecificData"
	Caption                  string
	Description              string
	ElementName              string
	ResourceType             uint16
	OtherResourceType        string
	ResourceSubType          string
	PoolID                   string
	ConsumerVisibility       uint16
	HostResource             []string
	AllocationUnits          string
	VirtualQuantity          uint64
	Reservation              uint64
	Limit                    uint64
	Weight                   uint32
	AutomaticAllocation      bool
	AutomaticDeallocation    bool
	Parent                   string
	Connection               []string
	Address                  string
	MappingBehavior          uint16
	AddressOnParent          string
	VirtualQuantityUnits     string   // = "count"
	VirtualSystemIdentifiers []string // = { "GUID" }
}

func (s *ResourceSettings) setParent(parent string) {
	s.Parent = parent
}

func (s *ResourceSettings) setAddressOnParent(address string) {
	s.AddressOnParent = address
}

func (s *ResourceSettings) Path() string {
	return s.S__PATH
}

func createResourceSettingGeneric(settings interface{}, resourceType string) (string, error) {
	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return "", err
	}

	ref, err := findResourceDefaults(service, resourceType)
	if err != nil {
		return "", err
	}

	resource, err := service.GetObject(ref)
	if err != nil {
		return "", err
	}

	defer resource.Close()
	resource, err = resource.CloneInstance()
	if err != nil {
		return "", err
	}
	defer resource.Close()

	if err = resource.PutAll(settings); err != nil {
		return "", err
	}

	return resource.GetCimText(), nil
}

func populateDefaults(subType string, settings interface{}) error {
	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return err
	}
	defer service.Close()

	ref, err := findResourceDefaults(service, subType)
	if err != nil {
		return err
	}

	return service.GetObjectAsObject(ref, settings)
}

func findResourceDefaults(service *wmiext.Service, subType string) (string, error) {
	wql := fmt.Sprintf("SELECT * FROM Msvm_AllocationCapabilities WHERE ResourceSubType = '%s'", subType)
	instance, err := service.FindFirstInstance(wql)
	if err != nil {
		return "", err
	}
	defer instance.Close()

	path, err := instance.Path()
	if err != nil {
		return "", err
	}

	enum, err := service.ExecQuery(fmt.Sprintf("references of {%s} where ResultClass = Msvm_SettingsDefineCapabilities", path))
	if err != nil {
		return "", err
	}
	defer enum.Close()

	for {
		entry, err := enum.Next()
		if err != nil {
			return "", err
		}
		if entry == nil {
			return "", errors.New("could not find settings definition for resource")
		}

		value, vErr := entry.GetAsUint("ValueRole")
		ref, pErr := entry.GetAsString("PartComponent")
		entry.Close()
		if vErr == nil && pErr == nil && value == 0 {
			return ref, nil
		}
	}
}
