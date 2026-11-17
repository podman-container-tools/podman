//go:build windows

package hypervctl

import (
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

type SystemSettingsBuilder struct {
	systemSettings    *SystemSettings
	processorSettings *ProcessorSettings
	memorySettings    *MemorySettings
	err               error
}

func NewSystemSettingsBuilder() *SystemSettingsBuilder {
	return &SystemSettingsBuilder{}
}

func (builder *SystemSettingsBuilder) PrepareSystemSettings(name string, beforeAdd func(*SystemSettings)) *SystemSettingsBuilder {
	if builder.err != nil {
		return builder
	}

	if builder.systemSettings == nil {
		settings := DefaultSystemSettings()
		settings.ElementName = name
		builder.systemSettings = settings
	}

	if beforeAdd != nil {
		beforeAdd(builder.systemSettings)
	}

	return builder
}

func (builder *SystemSettingsBuilder) PrepareProcessorSettings(beforeAdd func(*ProcessorSettings)) *SystemSettingsBuilder {
	if builder.err != nil {
		return builder
	}

	if builder.processorSettings == nil {
		settings, err := fetchDefaultProcessorSettings()
		if err != nil {
			builder.err = err
			return builder
		}
		builder.processorSettings = settings
	}

	if beforeAdd != nil {
		beforeAdd(builder.processorSettings)
	}

	return builder
}

func (builder *SystemSettingsBuilder) PrepareMemorySettings(beforeAdd func(*MemorySettings)) *SystemSettingsBuilder {
	if builder.err != nil {
		return builder
	}

	if builder.memorySettings == nil {
		settings, err := fetchDefaultMemorySettings()
		if err != nil {
			builder.err = err
			return builder
		}
		builder.memorySettings = settings
	}

	if beforeAdd != nil {
		beforeAdd(builder.memorySettings)
	}

	return builder
}

func (builder *SystemSettingsBuilder) Build() (*SystemSettings, error) {
	var service *wmiext.Service
	var err error

	if builder.PrepareSystemSettings("unnamed-vm", nil).
		PrepareProcessorSettings(nil).
		PrepareMemorySettings(nil).err != nil {
		return nil, err
	}

	if service, err = NewLocalHyperVService(); err != nil {
		return nil, err
	}
	defer service.Close()

	systemSettingsInst, err := service.SpawnInstance("Msvm_VirtualSystemSettingData")
	if err != nil {
		return nil, err
	}
	defer systemSettingsInst.Close()

	err = systemSettingsInst.PutAll(builder.systemSettings)
	if err != nil {
		return nil, err
	}

	memoryStr, err := createMemorySettings(builder.memorySettings)
	if err != nil {
		return nil, err
	}

	processorStr, err := createProcessorSettings(builder.processorSettings)
	if err != nil {
		return nil, err
	}

	vsms, err := service.GetSingletonInstance(VirtualSystemManagementService)
	if err != nil {
		return nil, err
	}
	defer vsms.Close()

	systemStr := systemSettingsInst.GetCimText()

	var job *wmiext.Instance
	var res int32
	var resultingSystem string
	err = vsms.BeginInvoke("DefineSystem").
		In("SystemSettings", systemStr).
		In("ResourceSettings", []string{memoryStr, processorStr}).
		Execute().
		Out("Job", &job).
		Out("ResultingSystem", &resultingSystem).
		Out("ReturnValue", &res).End()

	if err != nil {
		return nil, fmt.Errorf("failed to define system: %w", err)
	}

	err = waitVMResult(res, service, job, "failed to define system", nil)
	if err != nil {
		return nil, err
	}

	newSettings, err := service.FindFirstRelatedInstance(resultingSystem, "Msvm_VirtualSystemSettingData")
	if err != nil {
		return nil, err
	}
	path, err := newSettings.Path()
	if err != nil {
		return nil, err
	}

	if err = service.GetObjectAsObject(path, builder.systemSettings); err != nil {
		return nil, err
	}

	return builder.systemSettings, nil
}
