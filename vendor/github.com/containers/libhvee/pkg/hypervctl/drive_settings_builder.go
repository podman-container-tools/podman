//go:build windows

package hypervctl

type DriveSettingsBuilder struct {
	systemSettings *SystemSettings
	err            error
}

type ControllerSettingsBuilder struct {
	driveSettingsBuilder DriveSettingsBuilder
	controllerSettings   *ScsiControllerSettings
	err                  error
}

type SyntheticDiskDriveSettingsBuilder struct {
	controllerBuilder *ControllerSettingsBuilder
	driveSettings     *SyntheticDiskDriveSettings
	err               error
}

type SyntheticDvdDriveSettingsBuilder struct {
	controllerBuilder *ControllerSettingsBuilder
	driveSettings     *SyntheticDvdDriveSettings
	err               error
}

type VirtualHardDiskStorageSettingsBuilder struct {
	driveBuilder *SyntheticDiskDriveSettingsBuilder
	diskSettings *VirtualHardDiskStorageSettings
	err          error
}

type VirtualDvdDiskStorageSettingsBuilder struct {
	driveBuilder *SyntheticDvdDriveSettingsBuilder
	diskSettings *VirtualDvdDiskStorageSettings
	err          error
}

func NewDriveSettingsBuilder(systemSettings *SystemSettings) *DriveSettingsBuilder {
	return &DriveSettingsBuilder{systemSettings: systemSettings}
}

func (builder *DriveSettingsBuilder) AddScsiController() *ControllerSettingsBuilder {
	if builder.err != nil {
		return &ControllerSettingsBuilder{driveSettingsBuilder: *builder, err: builder.err}
	}

	controllerSettings, err := builder.systemSettings.AddScsiController()
	builder.setErr(err)

	return &ControllerSettingsBuilder{
		driveSettingsBuilder: *builder,
		controllerSettings:   controllerSettings,
		err:                  err,
	}
}

func (builder *ControllerSettingsBuilder) AddSyntheticDiskDrive(slot uint) *SyntheticDiskDriveSettingsBuilder {
	if builder.err != nil {
		return &SyntheticDiskDriveSettingsBuilder{controllerBuilder: builder, err: builder.err}
	}

	driveSettings, err := builder.controllerSettings.AddSyntheticDiskDrive(slot)
	builder.setErr(err)

	return &SyntheticDiskDriveSettingsBuilder{
		controllerBuilder: builder,
		driveSettings:     driveSettings,
		err:               err,
	}
}

func (builder *ControllerSettingsBuilder) AddSyntheticDvdDrive(slot uint) *SyntheticDvdDriveSettingsBuilder {
	if builder.err != nil {
		return &SyntheticDvdDriveSettingsBuilder{controllerBuilder: builder, err: builder.err}
	}

	driveSettings, err := builder.controllerSettings.AddSyntheticDvdDrive(slot)
	builder.setErr(err)

	return &SyntheticDvdDriveSettingsBuilder{
		controllerBuilder: builder,
		driveSettings:     driveSettings,
		err:               err,
	}
}

func (builder *SyntheticDiskDriveSettingsBuilder) DefineVirtualHardDisk(vhdxFile string, beforeAdd func(*VirtualHardDiskStorageSettings)) *VirtualHardDiskStorageSettingsBuilder {
	if builder.err != nil {
		return &VirtualHardDiskStorageSettingsBuilder{driveBuilder: builder, err: builder.err}
	}

	diskSettings, err := builder.driveSettings.DefineVirtualHardDisk(vhdxFile, beforeAdd)
	builder.setErr(err)

	return &VirtualHardDiskStorageSettingsBuilder{
		driveBuilder: builder,
		diskSettings: diskSettings,
		err:          err,
	}
}

func (builder *SyntheticDvdDriveSettingsBuilder) DefineVirtualDvdDisk(imageFile string) *VirtualDvdDiskStorageSettingsBuilder {
	if builder.err != nil {
		return &VirtualDvdDiskStorageSettingsBuilder{driveBuilder: builder, err: builder.err}
	}

	diskSettings, err := builder.driveSettings.DefineVirtualDvdDisk(imageFile)
	builder.setErr(err)

	return &VirtualDvdDiskStorageSettingsBuilder{
		driveBuilder: builder,
		diskSettings: diskSettings,
		err:          err,
	}
}

func (builder *SyntheticDvdDriveSettingsBuilder) setErr(err error) {
	builder.err = err
	builder.controllerBuilder.setErr(err)
}

func (builder *SyntheticDiskDriveSettingsBuilder) setErr(err error) {
	builder.err = err
	builder.controllerBuilder.setErr(err)
}

func (builder *ControllerSettingsBuilder) setErr(err error) {
	builder.err = err
	builder.driveSettingsBuilder.setErr(err)
}

func (builder *DriveSettingsBuilder) setErr(err error) {
	builder.err = err
}

func (builder *ControllerSettingsBuilder) Finish() *DriveSettingsBuilder {
	return &builder.driveSettingsBuilder
}

func (builder *VirtualHardDiskStorageSettingsBuilder) Finish() *SyntheticDiskDriveSettingsBuilder {
	return builder.driveBuilder
}

func (builder *VirtualDvdDiskStorageSettingsBuilder) Finish() *SyntheticDvdDriveSettingsBuilder {
	return builder.driveBuilder
}

func (builder *SyntheticDiskDriveSettingsBuilder) Finish() *ControllerSettingsBuilder {
	return builder.controllerBuilder
}

func (builder *SyntheticDvdDriveSettingsBuilder) Finish() *ControllerSettingsBuilder {
	return builder.controllerBuilder
}

func (builder *VirtualHardDiskStorageSettingsBuilder) Get(s **VirtualHardDiskStorageSettings) *VirtualHardDiskStorageSettingsBuilder {
	*s = builder.diskSettings
	return builder
}

func (builder *VirtualDvdDiskStorageSettingsBuilder) Get(s **VirtualDvdDiskStorageSettings) *VirtualDvdDiskStorageSettingsBuilder {
	*s = builder.diskSettings
	return builder
}

func (builder *SyntheticDiskDriveSettingsBuilder) Get(s **SyntheticDiskDriveSettings) *SyntheticDiskDriveSettingsBuilder {
	*s = builder.driveSettings
	return builder
}

func (builder *SyntheticDvdDriveSettingsBuilder) Get(s **SyntheticDvdDriveSettings) *SyntheticDvdDriveSettingsBuilder {
	*s = builder.driveSettings
	return builder
}

func (builder *ControllerSettingsBuilder) Get(s **ScsiControllerSettings) *ControllerSettingsBuilder {
	*s = builder.controllerSettings
	return builder
}

func (builder *DriveSettingsBuilder) Complete() error {
	return builder.err
}
