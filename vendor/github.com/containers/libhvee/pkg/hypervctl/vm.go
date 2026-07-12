//go:build windows

package hypervctl

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/containers/libhvee/pkg/kvp/ginsu"
	"github.com/containers/libhvee/pkg/wmiext"
)

// delete this when close to being done
var (
	ErrNotImplemented = errors.New("function not implemented")
)

type VirtualMachine struct {
	S__PATH                                  string `json:"-"`
	S__CLASS                                 string `json:"-"`
	InstanceID                               string
	Caption                                  string
	Description                              string
	ElementName                              string
	InstallDate                              time.Time
	OperationalStatus                        []uint16
	StatusDescriptions                       []string
	Status                                   string
	HealthState                              uint16
	CommunicationStatus                      uint16
	DetailedStatus                           uint16
	OperatingStatus                          uint16
	PrimaryStatus                            uint16
	EnabledState                             uint16
	OtherEnabledState                        string
	RequestedState                           uint16
	EnabledDefault                           uint16
	TimeOfLastStateChange                    string
	AvailableRequestedStates                 []uint16
	TransitioningToState                     uint16
	CreationClassName                        string
	Name                                     string
	PrimaryOwnerName                         string
	PrimaryOwnerContact                      string
	Roles                                    []string
	NameFormat                               string
	OtherIdentifyingInfo                     []string
	IdentifyingDescriptions                  []string
	Dedicated                                []uint16
	OtherDedicatedDescriptions               []string
	ResetCapability                          uint16
	PowerManagementCapabilities              []uint16
	OnTimeInMilliseconds                     uint64
	ProcessID                                uint32
	TimeOfLastConfigurationChange            string
	NumberOfNumaNodes                        uint16
	ReplicationState                         uint16
	ReplicationHealth                        uint16
	ReplicationMode                          uint16
	FailedOverReplicationType                uint16
	LastReplicationType                      uint16
	LastApplicationConsistentReplicationTime string
	LastReplicationTime                      time.Time
	LastSuccessfulBackupTime                 string
	EnhancedSessionModeState                 uint16
	vmm                                      *VirtualMachineManager
}

func (vm *VirtualMachine) Path() string {
	return vm.S__PATH
}
func (vm *VirtualMachine) SplitAndAddIgnition(keyPrefix string, ignRdr *bytes.Reader) error {
	parts, err := ginsu.Dice(ignRdr)
	if err != nil {
		return err
	}
	for idx, val := range parts {
		key := fmt.Sprintf("%s%d", keyPrefix, idx)
		if err := vm.AddKeyValuePair(key, val); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VirtualMachine) AddKeyValuePair(key string, value string) error {
	return vm.kvpOperation("AddKvpItems", key, value, false, "key already exists?")
}

func (vm *VirtualMachine) ModifyKeyValuePair(key string, value string) error {
	return vm.kvpOperation("ModifyKvpItems", key, value, false, "key invalid?")
}

func (vm *VirtualMachine) PutKeyValuePair(key string, value string) error {
	err := vm.AddKeyValuePair(key, value)
	kvpError, ok := err.(*KvpError)
	if !ok || kvpError.ErrorCode != KvpIllegalArgument {
		return err
	}

	return vm.ModifyKeyValuePair(key, value)
}

func (vm *VirtualMachine) RemoveKeyValuePair(key string) error {
	return vm.kvpOperation("RemoveKvpItems", key, "", false, "key invalid?")
}

func (vm *VirtualMachine) RemoveKeyValuePairNoWait(key string) error {
	return vm.kvpOperation("RemoveKvpItems", key, "", true, "key invalid?")
}

func (vm *VirtualMachine) GetKeyValuePairs() (map[string]string, error) {
	var service *wmiext.Service
	var err error

	if service, err = NewLocalHyperVService(); err != nil {
		return nil, err
	}

	defer service.Close()

	i, err := service.FindFirstRelatedInstance(vm.Path(), "Msvm_KvpExchangeComponent")
	if err != nil {
		return nil, err
	}

	defer i.Close()

	var path string
	path, err = i.GetAsString("__PATH")
	if err != nil {
		return nil, err

	}

	i, err = service.FindFirstRelatedInstance(path, "Msvm_KvpExchangeComponentSettingData")
	if err != nil {
		return nil, err
	}
	defer i.Close()

	s, err := i.GetAsString("HostExchangeItems")
	if err != nil {
		return nil, err
	}

	return parseKvpMapXml(s)
}

func (vm *VirtualMachine) kvpOperation(op string, key string, value string, nowait bool, illegalSuggestion string) error {
	var service *wmiext.Service
	var vsms, job *wmiext.Instance
	var ret int32
	var err error

	if service, err = NewLocalHyperVService(); err != nil {
		return err
	}
	defer service.Close()

	vsms, err = service.GetSingletonInstance(VirtualSystemManagementService)
	if err != nil {
		return err
	}
	defer vsms.Close()

	itemStr, err := createKvpItem(service, key, value)
	if err != nil {
		return err
	}

	execution := vsms.BeginInvoke(op).
		In("TargetSystem", vm.Path()).
		In("DataItems", []string{itemStr}).
		Execute().
		Out("ReturnValue", &ret).
		Out("Job", &job)

	if err := execution.End(); err != nil {
		return fmt.Errorf("%s execution failed: %w", op, err)
	}

	defer job.Close()
	if ret == 0 || (nowait && ret == 4096) {
		return nil
	}

	if ret == 4096 {
		err = wmiext.WaitJob(service, job)
	} else {
		err = &wmiext.JobError{ErrorCode: int(ret)}
	}

	if jobErr, ok := err.(*wmiext.JobError); ok && jobErr.Description != "" {
		return errors.New(jobErr.Error())
	}
	return translateKvpError(err, illegalSuggestion)
}

func waitVMResult(res int32, service *wmiext.Service, job *wmiext.Instance, errorMsg string, translate func(int) error) error {
	var err error

	switch res {
	case 0:
		return nil
	case 4096:
		err = wmiext.WaitJob(service, job)
		defer job.Close()
	default:
		if translate != nil {
			return translate(int(res))
		}

		return fmt.Errorf("%s (result code %d)", errorMsg, res)
	}

	if err != nil {
		if jobErr, ok := err.(*wmiext.JobError); ok && jobErr.Description != "" {
			return errors.New(jobErr.Error())
		}
	}

	return err
}

func (vm *VirtualMachine) StopWithForce() error {
	return vm.stop(true)
}

func (vm *VirtualMachine) Stop() error {
	return vm.stop(false)
}

func (vm *VirtualMachine) stop(force bool) error {
	if !Enabled.equal(vm.EnabledState) {
		return ErrMachineNotRunning
	}
	var (
		err error
		res int32
		srv *wmiext.Service
	)
	if srv, err = NewLocalHyperVService(); err != nil {
		return err
	}
	wmiInst, err := srv.FindFirstRelatedInstance(vm.Path(), "Msvm_ShutdownComponent")
	if err != nil {
		return err
	}
	// https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/msvm-shutdowncomponent-initiateshutdown
	err = wmiInst.BeginInvoke("InitiateShutdown").
		In("Reason", "User requested").
		In("Force", force).
		Execute().
		Out("ReturnValue", &res).End()
	if err != nil {
		return err
	}

	if res != 0 {
		return translateShutdownError(int(res))
	}

	// Wait for vm to actually *be* down
	for i := 0; i < 200; i++ {
		refreshVM, err := vm.vmm.GetMachine(vm.ElementName)
		if err != nil {
			return err
		}
		if refreshVM.State() == Disabled {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (vm *VirtualMachine) Start() error {
	var (
		srv *wmiext.Service
		err error
		job *wmiext.Instance
		res int32
	)

	if s := vm.EnabledState; !Disabled.equal(s) {
		if Enabled.equal(s) {
			return ErrMachineAlreadyRunning
		} else if Starting.equal(s) {
			return ErrMachineAlreadyRunning
		}
		return errors.New("machine not in a state to start")
	}

	if srv, err = getService(srv); err != nil {
		return err
	}
	defer srv.Close()

	instance, err := srv.GetObject(vm.Path())
	if err != nil {
		return err
	}
	defer instance.Close()

	// https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/cim-concretejob-requeststatechange
	if err := instance.BeginInvoke("RequestStateChange").
		In("RequestedState", uint16(start)).
		In("TimeoutPeriod", &time.Time{}).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &res).End(); err != nil {
		return err
	}
	return waitVMResult(res, srv, job, "failed to start vm", nil)
}

func getService(_ *wmiext.Service) (*wmiext.Service, error) {
	// any reason why when we instantiate a vm, we should NOT just embed a service?
	return NewLocalHyperVService()
}

func (vm *VirtualMachine) GetConfig(diskPath string) (*HyperVConfig, error) {
	var (
		diskSize uint64
	)
	summary, err := vm.GetSummaryInformation(SummaryRequestCommon)
	if err != nil {
		return nil, err
	}

	// Grabbing actual disk size
	diskPathInfo, err := os.Stat(diskPath)
	if err != nil {
		return nil, err
	}
	diskSize = uint64(diskPathInfo.Size())
	mem := MemorySettings{}
	if err := vm.getMemorySettings(&mem); err != nil {
		return nil, err
	}

	config := HyperVConfig{
		Hardware: HardwareConfig{
			// TODO we could implement a getProcessorSettings like we did for memory
			CPUs:     summary.NumberOfProcessors,
			DiskPath: diskPath,
			DiskSize: diskSize,
			Memory:   mem.Limit,
		},
		Status: Statuses{
			Created:  vm.InstallDate,
			LastUp:   time.Time{},
			Running:  Enabled.equal(vm.EnabledState),
			Starting: vm.IsStarting(),
			State:    EnabledState(vm.EnabledState),
		},
	}
	return &config, nil
}

// GetSummaryInformation returns the live VM summary information for this virtual machine.
// The requestedFields parameter controls which fields of summary information are populated.
// SummaryRequestCommon and SummaryRequestNearAll provide predefined combinations for this
// parameter
func (vm *VirtualMachine) GetSummaryInformation(requestedFields SummaryRequestSet) (*SummaryInformation, error) {
	service, err := NewLocalHyperVService()
	if err != nil {
		return nil, err
	}
	defer service.Close()

	instance, err := vm.fetchSystemSettingsInstance(service)
	if err != nil {
		return nil, err
	}
	defer instance.Close()

	path, err := instance.Path()
	if err != nil {
		return nil, err
	}

	result, err := vm.vmm.getSummaryInformation(path, requestedFields)
	if err != nil {
		return nil, err
	}

	if len(result) < 1 {
		return nil, errors.New("summary information search returned an empty result set")
	}

	return &result[0], nil
}

// NewVirtualMachine creates a new vm in hyperv
// decided to not return a *VirtualMachine here because of how Podman is
// likely to use this.  this could be easily added if desirable
func (vmm *VirtualMachineManager) NewVirtualMachine(name string, config *HardwareConfig) error {
	exists, err := vmm.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return ErrMachineAlreadyExists
	}

	// TODO I gotta believe there are naming restrictions for vms in hyperv?
	// TODO If something fails during creation, do we rip things down or follow precedent from other machines?  user deletes things

	systemSettings, err := NewSystemSettingsBuilder().
		PrepareSystemSettings(name, nil).
		PrepareMemorySettings(func(ms *MemorySettings) {
			//ms.DynamicMemoryEnabled = false
			//ms.VirtualQuantity = 8192 // Startup memory
			//ms.Reservation = config.Memory // min

			// The API seems to require both of these even
			// when not using dynamic memory
			ms.Limit = config.Memory
			ms.VirtualQuantity = config.Memory
		}).
		PrepareProcessorSettings(func(ps *ProcessorSettings) {
			ps.VirtualQuantity = uint64(config.CPUs) // 4 cores
		}).
		Build()
	if err != nil {
		return err
	}

	builder := NewDriveSettingsBuilder(systemSettings).
		AddScsiController().
		AddSyntheticDiskDrive(0).
		DefineVirtualHardDisk(config.DiskPath, func(vhdss *VirtualHardDiskStorageSettings) {
			// set extra params like
			// vhdss.IOPSLimit = 5000
		}).
		Finish(). // disk
		Finish()  // drive

	if config.DVDDiskPath != "" {
		// Add a DVD drive if the DVDDiskPath is set
		// This is useful for cloud-init or other bootable media
		builder = builder.
			AddSyntheticDvdDrive(1).
			DefineVirtualDvdDisk(config.DVDDiskPath).
			Finish(). // disk
			Finish()  // drive
	}

	if err := builder.
		Finish(). // controller
		Complete(); err != nil {
		return err
	}

	// Add default network connection
	if config.Network {
		if err := NewNetworkSettingsBuilder(systemSettings).
			AddSyntheticEthernetPort(nil).
			AddEthernetPortAllocation(""). // "" = connect to default switch
			Finish().                      // allocation
			Finish().                      // port
			Complete(); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VirtualMachine) fetchSystemSettingsInstance(service *wmiext.Service) (*wmiext.Instance, error) {
	// When a settings snapshot is taken there are multiple associations, use only the realized/active version
	return service.FindFirstRelatedInstanceThrough(vm.Path(), "Msvm_VirtualSystemSettingData", "Msvm_SettingsDefineState")
}

func (vm *VirtualMachine) fetchExistingResourceSettings(service *wmiext.Service, resourceType string, resourceSettings interface{}) error {
	const errFmt = "could not fetch resource settings (%s): %w"
	// When a settings snapshot is taken there are multiple associations, use only the realized/active version
	instance, err := vm.fetchSystemSettingsInstance(service)
	if err != nil {
		return fmt.Errorf(errFmt, resourceType, err)
	}
	defer instance.Close()

	path, err := instance.Path()
	if err != nil {
		return fmt.Errorf(errFmt, resourceType, err)

	}

	return service.FindFirstRelatedObject(path, resourceType, resourceSettings)
}

func (vm *VirtualMachine) getMemorySettings(m *MemorySettings) error {
	service, err := NewLocalHyperVService()
	if err != nil {
		return err
	}
	defer service.Close()
	return vm.fetchExistingResourceSettings(service, "Msvm_MemorySettingData", m)
}

// Update processor and/or mem
func (vm *VirtualMachine) UpdateProcessorMemSettings(updateProcessor func(*ProcessorSettings), updateMemory func(*MemorySettings)) error {
	service, err := NewLocalHyperVService()
	if err != nil {
		return err
	}
	defer service.Close()

	proc := &ProcessorSettings{}
	mem := &MemorySettings{}

	var settings []string
	if updateProcessor != nil {
		err = vm.fetchExistingResourceSettings(service, "Msvm_ProcessorSettingData", proc)
		if err != nil {
			return err
		}

		updateProcessor(proc)

		processorStr, err := createProcessorSettings(proc)
		if err != nil {
			return err
		}
		settings = append(settings, processorStr)
	}

	if updateMemory != nil {
		if err := vm.getMemorySettings(mem); err != nil {
			return err
		}

		updateMemory(mem)

		memStr, err := createMemorySettings(mem)
		if err != nil {
			return err
		}

		settings = append(settings, memStr)
	}

	if len(settings) < 1 {
		return nil
	}

	vsms, err := service.GetSingletonInstance("Msvm_VirtualSystemManagementService")
	if err != nil {
		return err
	}
	defer vsms.Close()

	var job *wmiext.Instance
	var res int32
	err = vsms.BeginInvoke("ModifyResourceSettings").
		In("ResourceSettings", settings).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &res).
		End()

	if err != nil {
		return fmt.Errorf("failed to modify resource settings: %w", err)
	}

	return waitVMResult(res, service, job, "failed to modify resource settings", translateModifyError)
}

func (vm *VirtualMachine) remove() (int32, error) {
	var (
		err error
		res int32
		srv *wmiext.Service
	)

	refreshVM, err := vm.vmm.GetMachine(vm.ElementName)
	if err != nil {
		return 0, err
	}
	// Check for disabled/stopped state
	if !Disabled.equal(refreshVM.EnabledState) {
		return -1, ErrMachineStateInvalid
	}
	if srv, err = NewLocalHyperVService(); err != nil {
		return -1, err
	}

	vsms, err := srv.GetSingletonInstance("Msvm_VirtualSystemManagementService")
	if err != nil {
		return -1, err
	}
	defer vsms.Close()

	var (
		job *wmiext.Instance
	)

	// https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/cim-virtualsystemmanagementservice-destroysystem
	if err := vsms.BeginInvoke("DestroySystem").
		In("AffectedSystem", vm.Path()).
		Execute().
		Out("Job", &job).
		Out("ReturnValue", &res).End(); err != nil {
		return -1, err
	}

	// do i have this correct? you can get an error without a result?
	if err := waitVMResult(res, srv, job, "failed to remove vm", nil); err != nil {
		return -1, err
	}
	return res, nil
}

func (vm *VirtualMachine) Remove(diskPath string) error {
	if _, err := vm.remove(); err != nil {
		return err
	}

	// Remove disk only if we were given one
	if len(diskPath) > 0 {
		if err := os.Remove(diskPath); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VirtualMachine) State() EnabledState {
	return EnabledState(vm.EnabledState)
}

func (vm *VirtualMachine) IsStarting() bool {
	return Starting.equal(vm.EnabledState)
}
