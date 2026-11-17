//go:build windows

package hypervctl

import (
	"errors"
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

const (
	HyperVNamespace                = "root\\virtualization\\v2"
	VirtualSystemManagementService = "Msvm_VirtualSystemManagementService"
	MsvmComputerSystem             = "Msvm_ComputerSystem"
)

// https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/msvm-computersystem

type VirtualMachineManager struct {
}

func NewVirtualMachineManager() *VirtualMachineManager {
	return &VirtualMachineManager{}
}

func NewLocalHyperVService() (*wmiext.Service, error) {
	service, err := wmiext.NewLocalService(HyperVNamespace)
	if err != nil {
		return nil, translateCommonHyperVWmiError(err)
	}

	return service, nil
}

func (vmm *VirtualMachineManager) GetAll() ([]*VirtualMachine, error) {
	// Fetch through settings to avoid locale sensitive properties
	const wql = "Select * From Msvm_VirtualSystemSettingData Where VirtualSystemType = 'Microsoft:Hyper-V:System:Realized'"

	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return []*VirtualMachine{}, err
	}
	defer service.Close()

	var enum *wmiext.Enum
	if enum, err = service.ExecQuery(wql); err != nil {
		return nil, err
	}
	defer enum.Close()
	var vms []*VirtualMachine

	for {
		settings, err := enum.Next()
		if err != nil {
			return vms, err
		}

		// Finished iterating
		if settings == nil {
			break
		}

		vm, err := vmm.findVMFromSettings(service, settings)
		settings.Close()
		if err != nil {
			return vms, err
		}

		vms = append(vms, vm)
	}

	return vms, nil
}

func (vmm *VirtualMachineManager) Exists(name string) (bool, error) {
	vms, err := vmm.GetAll()
	if err != nil {
		return false, err
	}
	for _, i := range vms {
		// TODO should case be honored or ignored?
		if i.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// GetMachine is a stub to lookup and get settings for a VM
func (vmm *VirtualMachineManager) GetMachine(name string) (*VirtualMachine, error) {
	return vmm.getMachine(name)
}

// GetMachineExists looks for a machine defined in hyperv and returns it if it exists
func (vmm *VirtualMachineManager) GetMachineExists(name string) (bool, *VirtualMachine, error) {
	vm, err := vmm.getMachine(name)
	if err != nil {
		if errors.Is(err, wmiext.ErrNoResults) {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, vm, nil
}

// getMachine looks up a single VM by name
func (vmm *VirtualMachineManager) getMachine(name string) (*VirtualMachine, error) {
	const wql = "Select * From Msvm_VirtualSystemSettingData Where VirtualSystemType = 'Microsoft:Hyper-V:System:Realized' And ElementName='%s'"

	vm := &VirtualMachine{}
	var service *wmiext.Service
	var err error

	if service, err = NewLocalHyperVService(); err != nil {
		return vm, err
	}
	defer service.Close()

	var enum *wmiext.Enum
	if enum, err = service.ExecQuery(fmt.Sprintf(wql, name)); err != nil {
		return nil, err
	}
	defer enum.Close()

	settings, err := service.FindFirstInstance(fmt.Sprintf(wql, name))
	if err != nil {
		if errors.Is(err, wmiext.ErrNoResults) {
			return nil, err
		}
		return vm, fmt.Errorf("could not find virtual machine %q: %w", name, err)
	}
	defer settings.Close()

	return vmm.findVMFromSettings(service, settings)
}

func (vmm *VirtualMachineManager) findVMFromSettings(service *wmiext.Service, settings *wmiext.Instance) (*VirtualMachine, error) {
	path, err := settings.Path()
	if err != nil {
		return nil, err
	}

	vm := &VirtualMachine{vmm: vmm}
	err = service.FindFirstRelatedObject(path, MsvmComputerSystem, vm)

	return vm, err
}

func (*VirtualMachineManager) CreateVhdxFile(path string, maxSize uint64) error {
	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return err
	}
	defer service.Close()

	settings := &VirtualHardDiskSettings{}
	settings.Format = 3
	settings.MaxInternalSize = maxSize
	settings.Type = 3
	settings.Path = path

	instance, err := service.CreateInstance("Msvm_VirtualHardDiskSettingData", settings)
	if err != nil {
		return err
	}
	defer instance.Close()
	settingsStr := instance.GetCimText()

	imms, err := service.GetSingletonInstance("Msvm_ImageManagementService")
	if err != nil {
		return err
	}
	defer imms.Close()

	var job *wmiext.Instance
	var ret int32
	err = imms.BeginInvoke("CreateVirtualHardDisk").
		In("VirtualDiskSettingData", settingsStr).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &ret).
		End()

	if err != nil {
		return fmt.Errorf("failed to create vhdx: %w", err)
	}

	return waitVMResult(ret, service, job, "failed to create vhdx", nil)
}

// GetSummaryInformation returns the live VM summary information for all virtual machines.
// The requestedFields parameter controls which fields of summary information are populated.
// SummaryRequestCommon and SummaryRequestNearAll provide predefined combinations for this
// parameter.
func (vmm *VirtualMachineManager) GetSummaryInformation(requestedFields SummaryRequestSet) ([]SummaryInformation, error) {
	return vmm.getSummaryInformation("", requestedFields)
}

func (vmm *VirtualMachineManager) getSummaryInformation(settingsPath string, requestedFields SummaryRequestSet) ([]SummaryInformation, error) {
	var service *wmiext.Service
	var err error
	if service, err = NewLocalHyperVService(); err != nil {
		return nil, err
	}
	defer service.Close()

	vsms, err := service.GetSingletonInstance(VirtualSystemManagementService)
	if err != nil {
		return nil, err
	}
	defer vsms.Close()

	var summary []SummaryInformation

	inv := vsms.BeginInvoke("GetSummaryInformation").
		In("RequestedInformation", []uint(requestedFields))

	if len(settingsPath) > 0 {
		inv.In("SettingData", []string{settingsPath})
	}

	err = inv.Execute().
		Out("SummaryInformation", &summary).
		End()

	if err != nil {
		return nil, err
	}

	return summary, nil
}
