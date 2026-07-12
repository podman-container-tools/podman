//go:build windows

package hypervctl

import "time"

const (
	SummaryRequestName                             = 0
	SummaryRequestElementName                      = 1
	SummaryRequestCreationTime                     = 2
	SummaryRequestNotes                            = 3
	SummaryRequestProcessors                       = 4
	SummaryRequestSmallThumbnail                   = 5
	SummaryRequestMediumThumbnail                  = 6
	SummaryRequestLargeThumbnail                   = 7
	SummaryRequestAllocatedGPU                     = 8
	SummaryRequestVirtualSwitchNames               = 9
	SummaryRequestVersion                          = 10
	SummaryRequestShielded                         = 11
	SummaryRequestEnabledState                     = 100
	SummaryRequestProcessorLoad                    = 101
	SummaryRequestProcessorLoadHistory             = 102
	SummaryRequestMemoryUsage                      = 103
	SummaryRequestHeartbeat                        = 104
	SummaryRequestUptime                           = 105
	SummaryRequestGuestOperatingSystem             = 106
	SummaryRequestSnapshots                        = 107
	SummaryRequestAsynchronousTasks                = 108
	SummaryRequestHealthState                      = 109
	SummaryRequestOperationalStatus                = 110
	SummaryRequestStatusDescriptions               = 111
	SummaryRequestMemoryAvailable                  = 112
	SummaryRequestMemoryBuffer                     = 113
	SummaryRequestReplicationMode                  = 114
	SummaryRequestReplicationState                 = 115
	SummaryRequestReplicationHealth                = 116
	SummaryRequestApplicationHealth                = 117
	SummaryRequestReplicationStateEx               = 118
	SummaryRequestReplicationHealthEx              = 119
	SummaryRequestSwapFilesInUse                   = 120
	SummaryRequestIntegrationServicesVersionState  = 121
	SummaryRequestReplicationProvider              = 122
	SummaryRequestMemorySpansPhysicalNumaNodes     = 123
	SummaryRequestIntegrationServicesVersionState2 = 132
	SummaryRequestOtherEnabledState                = 132
)

type SummaryRequestSet []uint

var (

	// SummaryRequestCommon includes a smaller subset of commonly used fields
	SummaryRequestCommon = SummaryRequestSet{
		SummaryRequestName,
		SummaryRequestElementName,
		SummaryRequestCreationTime,
		SummaryRequestNotes,
		SummaryRequestProcessors,
		SummaryRequestEnabledState,
		SummaryRequestProcessorLoad,
		SummaryRequestMemoryUsage,
		SummaryRequestHeartbeat,
		SummaryRequestUptime,
		SummaryRequestGuestOperatingSystem,
		SummaryRequestHealthState,
		SummaryRequestOperationalStatus,
		SummaryRequestStatusDescriptions,
		SummaryRequestMemoryAvailable,
		SummaryRequestMemoryBuffer,
		SummaryRequestSwapFilesInUse,
	}

	// SummaryRequestNearAll includes everything but load history and thumbnails
	SummaryRequestNearAll = SummaryRequestSet{
		SummaryRequestName,
		SummaryRequestElementName,
		SummaryRequestCreationTime,
		SummaryRequestNotes,
		SummaryRequestProcessors,
		SummaryRequestAllocatedGPU,
		SummaryRequestVirtualSwitchNames,
		SummaryRequestVersion,
		SummaryRequestShielded,
		SummaryRequestEnabledState,
		SummaryRequestProcessorLoad,
		SummaryRequestMemoryUsage,
		SummaryRequestHeartbeat,
		SummaryRequestUptime,
		SummaryRequestGuestOperatingSystem,
		SummaryRequestSnapshots,
		SummaryRequestAsynchronousTasks,
		SummaryRequestHealthState,
		SummaryRequestOperationalStatus,
		SummaryRequestStatusDescriptions,
		SummaryRequestMemoryAvailable,
		SummaryRequestMemoryBuffer,
		SummaryRequestReplicationMode,
		SummaryRequestReplicationState,
		SummaryRequestReplicationHealth,
		SummaryRequestApplicationHealth,
		SummaryRequestReplicationStateEx,
		SummaryRequestReplicationHealthEx,
		SummaryRequestSwapFilesInUse,
		SummaryRequestIntegrationServicesVersionState,
		SummaryRequestReplicationProvider,
		SummaryRequestMemorySpansPhysicalNumaNodes,
		SummaryRequestIntegrationServicesVersionState2,
		SummaryRequestOtherEnabledState,
	}
)

// SummaryInformation https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/msvm-summaryinformation
type SummaryInformation struct {
	InstanceID                      string
	AllocatedGPU                    string
	Shielded                        bool
	AsynchronousTasks               []ConcreteJob
	CreationTime                    time.Time
	ElementName                     string
	EnabledState                    uint16
	OtherEnabledState               string
	GuestOperatingSystem            string
	HealthState                     uint16
	Heartbeat                       uint16
	MemoryUsage                     uint64
	MemoryAvailable                 int32
	AvailableMemoryBuffer           int32
	SwapFilesInUse                  bool
	Name                            string
	Notes                           string
	Version                         string
	NumberOfProcessors              uint16
	OperationalStatus               []uint16
	ProcessorLoad                   uint16
	ProcessorLoadHistory            []uint16
	Snapshots                       []SystemSettings
	StatusDescriptions              []string
	ThumbnailImage                  []uint8
	ThumbnailImageHeight            uint16
	ThumbnailImageWidth             uint16
	UpTime                          uint64
	ReplicationState                uint16
	ReplicationStateEx              []uint16
	ReplicationHealth               uint16
	ReplicationHealthEx             []uint16
	ReplicationMode                 uint16
	TestReplicaSystem               string // REF to CIM_ComputerSystem
	ApplicationHealth               uint16
	IntegrationServicesVersionState uint16
	MemorySpansPhysicalNumaNodes    bool
	ReplicationProviderId           []string
	EnhancedSessionModeState        uint16
	VirtualSwitchNames              []string
	VirtualSystemSubType            string
	HostComputerSystemName          string
}

// CIM_ConcreteJob https://learn.microsoft.com/en-us/windows/win32/hyperv_v2/msvm-concretejob
type ConcreteJob struct {
	InstanceID              string
	Caption                 string
	Description             string
	ElementName             string
	InstallDate             time.Time
	Name                    string
	OperationalStatus       []uint16 // = { 2 }
	StatusDescriptions      []string // = { "OK" }
	Status                  string
	HealthState             uint16 // = 5
	CommunicationStatus     uint16
	DetailedStatus          uint16
	OperatingStatus         uint16
	PrimaryStatus           uint16
	JobStatus               string
	TimeSubmitted           time.Time
	ScheduledStartTime      time.Time
	StartTime               time.Time
	ElapsedTime             time.Duration
	JobRunTimes             uint32
	RunMonth                uint8
	RunDay                  int8
	RunDayOfWeek            int8
	RunStartInterval        time.Time
	LocalOrUtcTime          uint16
	UntilTime               time.Time
	Notify                  string
	Owner                   string
	Priority                uint32
	PercentComplete         uint16
	DeleteOnCompletion      bool
	ErrorCode               uint16
	ErrorDescription        string
	ErrorSummaryDescription string
	RecoveryAction          uint16
	OtherRecoveryAction     string
	JobState                uint16
	TimeOfLastStateChange   time.Time
	TimeBeforeRemoval       time.Duration // =
	Cancellable             bool
	JobType                 uint16
}
