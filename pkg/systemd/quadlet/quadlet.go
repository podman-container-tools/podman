package quadlet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.podman.io/podman/v6/pkg/specgenutilexternal"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/storage/pkg/regexp"
)

const (
	// Fixme should use
	// go.podman.io/podman/v6/libpod/define.AutoUpdateLabel
	// but it is causing bloat
	autoUpdateLabel = "io.containers.autoupdate"
	// Directory for temporary Quadlet files (sysadmin owned)
	UnitDirTemp = "/run/containers/systemd"
	// Directory for global Quadlet files (sysadmin owned)
	UnitDirAdmin = "/etc/containers/systemd"
	// Directory for global Quadlet files (distro owned)
	UnitDirDistro = "/usr/share/containers/systemd"

	// Names of commonly used systemd/quadlet group names
	ArtifactGroup   = "Artifact"
	ContainerGroup  = "Container"
	InstallGroup    = "Install"
	KubeGroup       = "Kube"
	NetworkGroup    = "Network"
	PodGroup        = "Pod"
	ServiceGroup    = "Service"
	UnitGroup       = "Unit"
	VolumeGroup     = "Volume"
	ImageGroup      = "Image"
	BuildGroup      = "Build"
	QuadletGroup    = "Quadlet"
	XArtifactGroup  = "X-Artifact"
	XContainerGroup = "X-Container"
	XKubeGroup      = "X-Kube"
	XNetworkGroup   = "X-Network"
	XPodGroup       = "X-Pod"
	XVolumeGroup    = "X-Volume"
	XImageGroup     = "X-Image"
	XBuildGroup     = "X-Build"
	XQuadletGroup   = "X-Quadlet"
)

// Systemd Unit file keys
const (
	ServiceKeyWorkingDirectory = "WorkingDirectory"
)

// All the supported quadlet keys
const (
	KeyAddCapability         = "AddCapability"
	KeyAddDevice             = "AddDevice"
	KeyAddHost               = "AddHost"
	KeyAllTags               = "AllTags"
	KeyAnnotation            = "Annotation"
	KeyAppArmor              = "AppArmor"
	KeyArch                  = "Arch"
	KeyArtifact              = "Artifact"
	KeyAuthFile              = "AuthFile"
	KeyAutoUpdate            = "AutoUpdate"
	KeyBuildArg              = "BuildArg"
	KeyCertDir               = "CertDir"
	KeyCgroupsMode           = "CgroupsMode"
	KeyConfigMap             = "ConfigMap"
	KeyContainerName         = "ContainerName"
	KeyContainersConfModule  = "ContainersConfModule"
	KeyCopy                  = "Copy"
	KeyCreds                 = "Creds"
	KeyDecryptionKey         = "DecryptionKey"
	KeyDefaultDependencies   = "DefaultDependencies"
	KeyDevice                = "Device"
	KeyDisableDNS            = "DisableDNS"
	KeyDNS                   = "DNS"
	KeyDNSOption             = "DNSOption"
	KeyDNSSearch             = "DNSSearch"
	KeyDriver                = "Driver"
	KeyDropCapability        = "DropCapability"
	KeyEntrypoint            = "Entrypoint"
	KeyEnvironment           = "Environment"
	KeyEnvironmentFile       = "EnvironmentFile"
	KeyEnvironmentHost       = "EnvironmentHost"
	KeyExec                  = "Exec"
	KeyExitCodePropagation   = "ExitCodePropagation"
	KeyExitPolicy            = "ExitPolicy"
	KeyExposeHostPort        = "ExposeHostPort"
	KeyFile                  = "File"
	KeyForceRM               = "ForceRM"
	KeyGateway               = "Gateway"
	KeyGID                   = "GID"
	KeyGIDMap                = "GIDMap"
	KeyGlobalArgs            = "GlobalArgs"
	KeyGroup                 = "Group"
	KeyGroupAdd              = "GroupAdd"
	KeyHealthCmd             = "HealthCmd"
	KeyHealthInterval        = "HealthInterval"
	KeyHealthLogDestination  = "HealthLogDestination"
	KeyHealthMaxLogCount     = "HealthMaxLogCount"
	KeyHealthMaxLogSize      = "HealthMaxLogSize"
	KeyHealthOnFailure       = "HealthOnFailure"
	KeyHealthRetries         = "HealthRetries"
	KeyHealthStartPeriod     = "HealthStartPeriod"
	KeyHealthStartupCmd      = "HealthStartupCmd"
	KeyHealthStartupInterval = "HealthStartupInterval"
	KeyHealthStartupRetries  = "HealthStartupRetries"
	KeyHealthStartupSuccess  = "HealthStartupSuccess"
	KeyHealthStartupTimeout  = "HealthStartupTimeout"
	KeyHealthTimeout         = "HealthTimeout"
	KeyHostName              = "HostName"
	KeyHttpProxy             = "HttpProxy"
	KeyIgnoreFile            = "IgnoreFile"
	KeyImage                 = "Image"
	KeyImageTag              = "ImageTag"
	KeyInterfaceName         = "InterfaceName"
	KeyInternal              = "Internal"
	KeyIP                    = "IP"
	KeyIP6                   = "IP6"
	KeyIPAMDriver            = "IPAMDriver"
	KeyIPRange               = "IPRange"
	KeyIPv6                  = "IPv6"
	KeyKubeDownForce         = "KubeDownForce"
	KeyLabel                 = "Label"
	KeyLogDriver             = "LogDriver"
	KeyLogOpt                = "LogOpt"
	KeyMask                  = "Mask"
	KeyMemory                = "Memory"
	KeyMount                 = "Mount"
	KeyNetwork               = "Network"
	KeyNetworkAlias          = "NetworkAlias"
	KeyNetworkDeleteOnStop   = "NetworkDeleteOnStop"
	KeyNetworkName           = "NetworkName"
	KeyNoNewPrivileges       = "NoNewPrivileges"
	KeyNotify                = "Notify"
	KeyOptions               = "Options"
	KeyOS                    = "OS"
	KeyPidsLimit             = "PidsLimit"
	KeyPod                   = "Pod"
	KeyPodmanArgs            = "PodmanArgs"
	KeyPodName               = "PodName"
	KeyPolicy                = "Policy"
	KeyPublishPort           = "PublishPort"
	KeyPull                  = "Pull"
	KeyQuiet                 = "Quiet"
	KeyReadOnly              = "ReadOnly"
	KeyReadOnlyTmpfs         = "ReadOnlyTmpfs"
	KeyReloadCmd             = "ReloadCmd"
	KeyReloadSignal          = "ReloadSignal"
	KeyRemapGid              = "RemapGid"     // deprecated
	KeyRemapUid              = "RemapUid"     // deprecated
	KeyRemapUidSize          = "RemapUidSize" // deprecated
	KeyRemapUsers            = "RemapUsers"   // deprecated
	KeyRetry                 = "Retry"
	KeyRetryDelay            = "RetryDelay"
	KeyRootfs                = "Rootfs"
	KeyRunInit               = "RunInit"
	KeySeccompProfile        = "SeccompProfile"
	KeySecret                = "Secret"
	KeySecurityLabelDisable  = "SecurityLabelDisable"
	KeySecurityLabelFileType = "SecurityLabelFileType"
	KeySecurityLabelLevel    = "SecurityLabelLevel"
	KeySecurityLabelNested   = "SecurityLabelNested"
	KeySecurityLabelType     = "SecurityLabelType"
	KeyServiceName           = "ServiceName"
	KeySetWorkingDirectory   = "SetWorkingDirectory"
	KeyShmSize               = "ShmSize"
	KeyStartWithPod          = "StartWithPod"
	KeyStopSignal            = "StopSignal"
	KeyStopTimeout           = "StopTimeout"
	KeySubGIDMap             = "SubGIDMap"
	KeySubnet                = "Subnet"
	KeySubUIDMap             = "SubUIDMap"
	KeySysctl                = "Sysctl"
	KeyTarget                = "Target"
	KeyTimezone              = "Timezone"
	KeyTLSVerify             = "TLSVerify"
	KeyTmpfs                 = "Tmpfs"
	KeyType                  = "Type"
	KeyUID                   = "UID"
	KeyUIDMap                = "UIDMap"
	KeyUlimit                = "Ulimit"
	KeyUnmask                = "Unmask"
	KeyUser                  = "User"
	KeyUserNS                = "UserNS"
	KeyVariant               = "Variant"
	KeyVolatileTmp           = "VolatileTmp" // deprecated
	KeyVolume                = "Volume"
	KeyVolumeName            = "VolumeName"
	KeyWorkingDir            = "WorkingDir"
	KeyYaml                  = "Yaml"
)

// Unsupported keys in the Service group. Defined here so we can error when they are found
var UnsupportedServiceKeys = [...]string{"User", "Group", "DynamicUser"}

type UnitInfo struct {
	// The name of the generated systemd service unit
	ServiceName string
	// The name of the podman resource created by the service
	ResourceName string

	// For .pod units
	// List of containers to start with the pod
	ContainersToStart []string
}

type GroupInfo struct {
	// The name of the group in the original Quadlet file
	GroupName string
	// The x-group name to use in the target Service file
	XGroupName string
	// List of supported Keys for the group
	SupportedKeys map[string]bool
}

var (
	URL            = regexp.Delayed(`^((https?)|(git)://)|(github\.com/).+$`)
	validPortRange = regexp.Delayed(`\d+(-\d+)?(/udp|/tcp)?$`)

	unitDependencyKeys = []string{
		"After",
		"Before",
		"BindsTo",
		"Conflicts",
		"OnFailure",
		"OnSuccess",
		"PartOf",
		"PropagatesReloadTo",
		"PropagatesStopTo",
		"ReloadPropagatedFrom",
		"Requires",
		"Requisite",
		"StopPropagatedFrom",
		"Upholds",
		"Wants",
	}

	groupsInfo = map[string]GroupInfo{
		ContainerGroup: {
			GroupName:  ContainerGroup,
			XGroupName: XContainerGroup,
			SupportedKeys: map[string]bool{
				KeyAddCapability:         true,
				KeyAddDevice:             true,
				KeyAddHost:               true,
				KeyAnnotation:            true,
				KeyAppArmor:              true,
				KeyAutoUpdate:            true,
				KeyCgroupsMode:           true,
				KeyContainerName:         true,
				KeyContainersConfModule:  true,
				KeyDNS:                   true,
				KeyDNSOption:             true,
				KeyDNSSearch:             true,
				KeyDropCapability:        true,
				KeyEnvironment:           true,
				KeyEnvironmentFile:       true,
				KeyEnvironmentHost:       true,
				KeyEntrypoint:            true,
				KeyExec:                  true,
				KeyExposeHostPort:        true,
				KeyGIDMap:                true,
				KeyGlobalArgs:            true,
				KeyGroup:                 true,
				KeyGroupAdd:              true,
				KeyHealthCmd:             true,
				KeyHealthInterval:        true,
				KeyHealthOnFailure:       true,
				KeyHealthLogDestination:  true,
				KeyHealthMaxLogCount:     true,
				KeyHealthMaxLogSize:      true,
				KeyHealthRetries:         true,
				KeyHealthStartPeriod:     true,
				KeyHealthStartupCmd:      true,
				KeyHealthStartupInterval: true,
				KeyHealthStartupRetries:  true,
				KeyHealthStartupSuccess:  true,
				KeyHealthStartupTimeout:  true,
				KeyHealthTimeout:         true,
				KeyHostName:              true,
				KeyHttpProxy:             true,
				KeyIP6:                   true,
				KeyIP:                    true,
				KeyImage:                 true,
				KeyLabel:                 true,
				KeyLogDriver:             true,
				KeyLogOpt:                true,
				KeyMask:                  true,
				KeyMemory:                true,
				KeyMount:                 true,
				KeyNetwork:               true,
				KeyNetworkAlias:          true,
				KeyNoNewPrivileges:       true,
				KeyNotify:                true,
				KeyPidsLimit:             true,
				KeyPod:                   true,
				KeyPodmanArgs:            true,
				KeyPublishPort:           true,
				KeyPull:                  true,
				KeyReadOnly:              true,
				KeyReadOnlyTmpfs:         true,
				KeyReloadCmd:             true,
				KeyReloadSignal:          true,
				KeyRemapGid:              true,
				KeyRemapUid:              true,
				KeyRemapUidSize:          true,
				KeyRemapUsers:            true,
				KeyRetry:                 true,
				KeyRetryDelay:            true,
				KeyRootfs:                true,
				KeyRunInit:               true,
				KeySeccompProfile:        true,
				KeySecret:                true,
				KeySecurityLabelDisable:  true,
				KeySecurityLabelFileType: true,
				KeySecurityLabelLevel:    true,
				KeySecurityLabelNested:   true,
				KeySecurityLabelType:     true,
				KeyServiceName:           true,
				KeyShmSize:               true,
				KeyStopSignal:            true,
				KeyStartWithPod:          true,
				KeyStopTimeout:           true,
				KeySubGIDMap:             true,
				KeySubUIDMap:             true,
				KeySysctl:                true,
				KeyTimezone:              true,
				KeyTmpfs:                 true,
				KeyUIDMap:                true,
				KeyUlimit:                true,
				KeyUnmask:                true,
				KeyUser:                  true,
				KeyUserNS:                true,
				KeyVolatileTmp:           true,
				KeyVolume:                true,
				KeyWorkingDir:            true,
			},
		},
		VolumeGroup: {
			GroupName:  VolumeGroup,
			XGroupName: XVolumeGroup,
			SupportedKeys: map[string]bool{
				KeyContainersConfModule: true,
				KeyCopy:                 true,
				KeyDevice:               true,
				KeyDriver:               true,
				KeyGlobalArgs:           true,
				KeyGroup:                true,
				KeyImage:                true,
				KeyLabel:                true,
				KeyOptions:              true,
				KeyPodmanArgs:           true,
				KeyServiceName:          true,
				KeyType:                 true,
				KeyUser:                 true,
				KeyVolumeName:           true,
				KeyUID:                  true,
				KeyGID:                  true,
			},
		},
		NetworkGroup: {
			GroupName:  NetworkGroup,
			XGroupName: XNetworkGroup,
			SupportedKeys: map[string]bool{
				KeyLabel:                true,
				KeyDNS:                  true,
				KeyContainersConfModule: true,
				KeyGlobalArgs:           true,
				KeyDisableDNS:           true,
				KeyDriver:               true,
				KeyGateway:              true,
				KeyIPAMDriver:           true,
				KeyIPRange:              true,
				KeyIPv6:                 true,
				KeyInterfaceName:        true,
				KeyInternal:             true,
				KeyNetworkName:          true,
				KeyNetworkDeleteOnStop:  true,
				KeyOptions:              true,
				KeyServiceName:          true,
				KeySubnet:               true,
				KeyPodmanArgs:           true,
			},
		},
		KubeGroup: {
			GroupName:  KubeGroup,
			XGroupName: XKubeGroup,
			SupportedKeys: map[string]bool{
				KeyAutoUpdate:           true,
				KeyConfigMap:            true,
				KeyContainersConfModule: true,
				KeyExitCodePropagation:  true,
				KeyGlobalArgs:           true,
				KeyKubeDownForce:        true,
				KeyLogDriver:            true,
				KeyLogOpt:               true,
				KeyNetwork:              true,
				KeyPodmanArgs:           true,
				KeyPublishPort:          true,
				KeyRemapGid:             true,
				KeyRemapUid:             true,
				KeyRemapUidSize:         true,
				KeyRemapUsers:           true,
				KeyServiceName:          true,
				KeySetWorkingDirectory:  true,
				KeyUserNS:               true,
				KeyYaml:                 true,
			},
		},
		ImageGroup: {
			GroupName:  ImageGroup,
			XGroupName: XImageGroup,
			SupportedKeys: map[string]bool{
				KeyAllTags:              true,
				KeyArch:                 true,
				KeyAuthFile:             true,
				KeyCertDir:              true,
				KeyContainersConfModule: true,
				KeyCreds:                true,
				KeyDecryptionKey:        true,
				KeyGlobalArgs:           true,
				KeyImage:                true,
				KeyImageTag:             true,
				KeyOS:                   true,
				KeyPodmanArgs:           true,
				KeyPolicy:               true,
				KeyRetry:                true,
				KeyRetryDelay:           true,
				KeyServiceName:          true,
				KeyTLSVerify:            true,
				KeyVariant:              true,
			},
		},
		BuildGroup: {
			GroupName:  BuildGroup,
			XGroupName: XBuildGroup,
			SupportedKeys: map[string]bool{
				KeyAnnotation:           true,
				KeyArch:                 true,
				KeyAuthFile:             true,
				KeyBuildArg:             true,
				KeyContainersConfModule: true,
				KeyDNS:                  true,
				KeyDNSOption:            true,
				KeyDNSSearch:            true,
				KeyEnvironment:          true,
				KeyFile:                 true,
				KeyForceRM:              true,
				KeyGlobalArgs:           true,
				KeyGroupAdd:             true,
				KeyIgnoreFile:           true,
				KeyImageTag:             true,
				KeyLabel:                true,
				KeyNetwork:              true,
				KeyPodmanArgs:           true,
				KeyPull:                 true,
				KeyRetry:                true,
				KeyRetryDelay:           true,
				KeySecret:               true,
				KeyServiceName:          true,
				KeySetWorkingDirectory:  true,
				KeyTarget:               true,
				KeyTLSVerify:            true,
				KeyVariant:              true,
				KeyVolume:               true,
			},
		},
		ArtifactGroup: {
			GroupName:  ArtifactGroup,
			XGroupName: XArtifactGroup,
			SupportedKeys: map[string]bool{
				KeyArtifact:             true,
				KeyAuthFile:             true,
				KeyCertDir:              true,
				KeyContainersConfModule: true,
				KeyCreds:                true,
				KeyDecryptionKey:        true,
				KeyGlobalArgs:           true,
				KeyPodmanArgs:           true,
				KeyQuiet:                true,
				KeyRetry:                true,
				KeyRetryDelay:           true,
				KeyServiceName:          true,
				KeyTLSVerify:            true,
			},
		},
		PodGroup: {
			GroupName:  PodGroup,
			XGroupName: XPodGroup,
			SupportedKeys: map[string]bool{
				KeyAddHost:              true,
				KeyContainersConfModule: true,
				KeyDNS:                  true,
				KeyDNSOption:            true,
				KeyDNSSearch:            true,
				KeyExitPolicy:           true,
				KeyGIDMap:               true,
				KeyGlobalArgs:           true,
				KeyHostName:             true,
				KeyIP:                   true,
				KeyIP6:                  true,
				KeyLabel:                true,
				KeyNetwork:              true,
				KeyNetworkAlias:         true,
				KeyPodName:              true,
				KeyPodmanArgs:           true,
				KeyPublishPort:          true,
				KeyRemapGid:             true,
				KeyRemapUid:             true,
				KeyRemapUidSize:         true,
				KeyRemapUsers:           true,
				KeyServiceName:          true,
				KeyShmSize:              true,
				KeyStopTimeout:          true,
				KeySubGIDMap:            true,
				KeySubUIDMap:            true,
				KeyUIDMap:               true,
				KeyUserNS:               true,
				KeyVolume:               true,
			},
		},
	}

	// Supported keys in "Quadlet" group
	supportedQuadletKeys = map[string]bool{
		KeyDefaultDependencies: true,
	}
)

func (u *UnitInfo) ServiceFileName() string {
	return fmt.Sprintf("%s.service", u.ServiceName)
}

func removeExtension(name string, extraPrefix string, extraSuffix string) string {
	baseName := name

	dot := strings.LastIndexByte(name, '.')
	if dot > 0 {
		baseName = name[:dot]
	}

	return extraPrefix + baseName + extraSuffix
}

func isURL(urlCandidate string) bool {
	return URL.MatchString(urlCandidate)
}

func isPortRange(port string) bool {
	return validPortRange.MatchString(port)
}

func checkForUnknownKeysInSpecificGroup(unit *parser.UnitFile, groupName string, supportedKeys map[string]bool) error {
	keys := unit.ListKeys(groupName)
	for _, key := range keys {
		if !supportedKeys[key] {
			return fmt.Errorf("unsupported key '%s' in group '%s' in %s", key, groupName, unit.Path)
		}
	}

	return nil
}

func checkForUnknownKeys(unit *parser.UnitFile, groupName string, supportedKeys map[string]bool) error {
	err := checkForUnknownKeysInSpecificGroup(unit, groupName, supportedKeys)
	if err == nil {
		return checkForUnknownKeysInSpecificGroup(unit, QuadletGroup, supportedQuadletKeys)
	}

	return err
}

func usernsOpts(kind string, opts []string) string {
	var res strings.Builder
	res.WriteString(kind)
	if len(opts) > 0 {
		res.WriteString(":")
	}
	for i, opt := range opts {
		if i != 0 {
			res.WriteString(",")
		}
		res.WriteString(opt)
	}
	return res.String()
}

// Convert a quadlet container file (unit file with a Container group) to a systemd
// service file (unit file with Service group) based on the options in the
// Container group.
// The original Container group is kept around as X-Container.
func ConvertContainer(container *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error, error) {
	var warn, warnings error

	service, _, err := initServiceUnitFile(container, isUser, unitsInfoMap, ContainerGroup)
	if err != nil {
		return nil, warnings, err
	}

	// One image or rootfs must be specified for the container
	image, _ := container.Lookup(ContainerGroup, KeyImage)
	rootfs, _ := container.Lookup(ContainerGroup, KeyRootfs)
	if len(image) == 0 && len(rootfs) == 0 {
		return nil, warnings, fmt.Errorf("no Image or Rootfs key specified")
	}
	if len(image) > 0 && len(rootfs) > 0 {
		return nil, warnings, fmt.Errorf("the Image And Rootfs keys conflict can not be specified together")
	}

	if len(image) > 0 {
		var err error
		if image, err = handleImageSource(image, service, unitsInfoMap); err != nil {
			return nil, warnings, err
		}
	}

	containerName := getContainerName(container)

	// Set PODMAN_SYSTEMD_UNIT so that podman auto-update can restart the service.
	service.Add(ServiceGroup, "Environment", "PODMAN_SYSTEMD_UNIT=%n")

	// Only allow mixed or control-group, as nothing else works well
	killMode, ok := service.Lookup(ServiceGroup, "KillMode")
	if !ok || (killMode != "mixed" && killMode != "control-group") {
		if ok {
			return nil, warnings, fmt.Errorf("invalid KillMode '%s'", killMode)
		}

		// We default to mixed instead of control-group, because it lets conmon do its thing
		service.Set(ServiceGroup, "KillMode", "mixed")
	}

	// If conmon exited uncleanly it may not have removed the container, so
	// force it, -i makes it ignore non-existing files.
	serviceStopCmd := createBasePodmanCommand(container, ContainerGroup)
	serviceStopCmd.add("rm", "-v", "-f", "-i", containerName)
	service.AddCmdline(ServiceGroup, "ExecStop", serviceStopCmd.Args)
	// The ExecStopPost is needed when the main PID (i.e., conmon) gets killed.
	// In that case, ExecStop is not executed but *Post only.
	serviceStopCmd.Args[0] = fmt.Sprintf("-%s", serviceStopCmd.Args[0])
	service.AddCmdline(ServiceGroup, "ExecStopPost", serviceStopCmd.Args)

	warn, err = handleExecReload(container, service, ContainerGroup, containerName)
	warnings = errors.Join(warnings, warn)
	if err != nil {
		return nil, warnings, err
	}

	podman := createBasePodmanCommand(container, ContainerGroup)

	podman.add("run")

	podman.add("--name", containerName)

	podman.add(
		// And replace any previous container with the same name, not fail
		"--replace",

		// On clean shutdown, remove container
		"--rm",
	)

	handleLogDriver(container, ContainerGroup, podman)
	handleLogOpt(container, ContainerGroup, podman)

	// We delegate groups to the runtime
	service.Add(ServiceGroup, "Delegate", "yes")

	if cgroupsMode, ok := container.Lookup(ContainerGroup, KeyCgroupsMode); ok && len(cgroupsMode) > 0 {
		podman.add("--cgroups", cgroupsMode)
	} else {
		podman.add("--cgroups=split")
	}

	// Entrypoint needs special handling: an empty value is valid and means
	// "clear the image entrypoint" (podman run --entrypoint ""), so it
	// cannot go through lookupAndAddString which skips empty values.
	if val, ok := container.Lookup(ContainerGroup, KeyEntrypoint); ok {
		podman.addf("--entrypoint=%s", val)
	}

	stringKeys := map[string]string{
		KeyTimezone:    "--tz",
		KeyPidsLimit:   "--pids-limit",
		KeyShmSize:     "--shm-size",
		KeyWorkingDir:  "--workdir",
		KeyIP:          "--ip",
		KeyIP6:         "--ip6",
		KeyHostName:    "--hostname",
		KeyStopSignal:  "--stop-signal",
		KeyStopTimeout: "--stop-timeout",
		KeyPull:        "--pull",
		KeyMemory:      "--memory",
		KeyRetry:       "--retry",
		KeyRetryDelay:  "--retry-delay",
	}
	lookupAndAddString(container, ContainerGroup, stringKeys, podman)

	allStringsKeys := map[string]string{
		KeyNetworkAlias: "--network-alias",
		KeyUlimit:       "--ulimit",
		KeyDNS:          "--dns",
		KeyDNSOption:    "--dns-option",
		KeyDNSSearch:    "--dns-search",
		KeyGroupAdd:     "--group-add",
		KeyAddHost:      "--add-host",
		KeyTmpfs:        "--tmpfs",
	}
	lookupAndAddAllStrings(container, ContainerGroup, allStringsKeys, podman)

	boolKeys := map[string]string{
		KeyRunInit:         "--init",
		KeyEnvironmentHost: "--env-host",
		KeyHttpProxy:       "--http-proxy",
		KeyReadOnlyTmpfs:   "--read-only-tmpfs",
	}
	lookupAndAddBoolean(container, ContainerGroup, boolKeys, podman)

	if err := addNetworks(container, ContainerGroup, service, unitsInfoMap, podman); err != nil {
		return nil, warnings, err
	}

	serviceType, ok := service.Lookup(ServiceGroup, "Type")
	if ok && serviceType != "notify" && serviceType != "oneshot" {
		return nil, warnings, fmt.Errorf("invalid service Type '%s'", serviceType)
	}

	if serviceType != "oneshot" {
		// If we're not in oneshot mode always use some form of sd-notify, normally via conmon,
		// but we also allow passing it to the container by setting Notify=yes
		notify, ok := container.Lookup(ContainerGroup, KeyNotify)
		switch {
		case ok && strings.EqualFold(notify, "healthy"):
			podman.add("--sdnotify=healthy")
		case container.LookupBooleanWithDefault(ContainerGroup, KeyNotify, false):
			podman.add("--sdnotify=container")
		default:
			podman.add("--sdnotify=conmon")
		}
		service.Setv(ServiceGroup,
			"Type", "notify",
			"NotifyAccess", "all")

		// Detach from container, we don't need the podman process to hang around
		podman.add("-d")
	}

	if !container.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	// Default to no higher level privileges or caps
	noNewPrivileges := container.LookupBooleanWithDefault(ContainerGroup, KeyNoNewPrivileges, false)
	if noNewPrivileges {
		podman.add("--security-opt=no-new-privileges")
	}

	securityLabelDisable := container.LookupBooleanWithDefault(ContainerGroup, KeySecurityLabelDisable, false)
	if securityLabelDisable {
		podman.add("--security-opt", "label=disable")
	}

	securityLabelNested := container.LookupBooleanWithDefault(ContainerGroup, KeySecurityLabelNested, false)
	if securityLabelNested {
		podman.add("--security-opt", "label=nested")
	}

	securityLabelType, ok := container.Lookup(ContainerGroup, KeySecurityLabelType)
	if ok && len(securityLabelType) > 0 {
		podman.add("--security-opt", fmt.Sprintf("label=type:%s", securityLabelType))
	}

	securityLabelFileType, ok := container.Lookup(ContainerGroup, KeySecurityLabelFileType)
	if ok && len(securityLabelFileType) > 0 {
		podman.add("--security-opt", fmt.Sprintf("label=filetype:%s", securityLabelFileType))
	}

	securityLabelLevel, ok := container.Lookup(ContainerGroup, KeySecurityLabelLevel)
	if ok && len(securityLabelLevel) > 0 {
		podman.add("--security-opt", fmt.Sprintf("label=level:%s", securityLabelLevel))
	}

	apparmor, hasApparmor := container.Lookup(ContainerGroup, KeyAppArmor)
	if hasApparmor && len(apparmor) > 0 {
		podman.add("--security-opt", fmt.Sprintf("apparmor=%s", apparmor))
	}

	devices := container.LookupAllStrv(ContainerGroup, KeyAddDevice)
	for _, device := range devices {
		if device[0] == '-' {
			device = device[1:]
			file, _, _ := strings.Cut(device, ":")
			_, err := os.Stat(file)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		podman.add("--device", device)
	}

	// Default to no higher level privileges or caps
	seccompProfile, hasSeccompProfile := container.Lookup(ContainerGroup, KeySeccompProfile)
	if hasSeccompProfile {
		podman.add("--security-opt", fmt.Sprintf("seccomp=%s", seccompProfile))
	}

	dropCaps := container.LookupAllStrv(ContainerGroup, KeyDropCapability)

	for _, caps := range dropCaps {
		podman.add("--cap-drop", strings.ToLower(caps))
	}

	// But allow overrides with AddCapability
	addCaps := container.LookupAllStrv(ContainerGroup, KeyAddCapability)
	for _, caps := range addCaps {
		podman.add("--cap-add", strings.ToLower(caps))
	}

	sysctl := container.LookupAllStrv(ContainerGroup, KeySysctl)
	for _, sysctlItem := range sysctl {
		podman.add("--sysctl", sysctlItem)
	}

	// This was not moved to the generic handling since readOnly is used also with volatileTmp
	readOnly, ok := container.LookupBoolean(ContainerGroup, KeyReadOnly)
	if ok {
		podman.addBool("--read-only", readOnly)
	}

	volatileTmp := container.LookupBooleanWithDefault(ContainerGroup, KeyVolatileTmp, false)
	if volatileTmp && !readOnly {
		podman.add("--tmpfs", "/tmp:rw,size=512M,mode=1777")
	}

	if err := handleUser(container, ContainerGroup, podman); err != nil {
		return nil, warnings, err
	}

	if err := handleUserMappings(container, ContainerGroup, podman, true); err != nil {
		return nil, warnings, err
	}

	if err := addVolumes(container, service, ContainerGroup, unitsInfoMap, podman); err != nil {
		return nil, warnings, err
	}

	update, ok := container.Lookup(ContainerGroup, KeyAutoUpdate)
	if ok && len(update) > 0 {
		podman.addKeys(
			"--label",
			map[string]*string{
				autoUpdateLabel: &update,
			},
		)
	}

	exposedPorts := container.LookupAll(ContainerGroup, KeyExposeHostPort)
	for _, exposedPort := range exposedPorts {
		exposedPort = strings.TrimSpace(exposedPort) // Allow whitespace after

		if !isPortRange(exposedPort) {
			return nil, warnings, fmt.Errorf("invalid port format '%s'", exposedPort)
		}

		podman.add("--expose", exposedPort)
	}

	handlePublishPorts(container, ContainerGroup, podman)

	keyValKeys := map[string]string{
		KeyEnvironment: "--env",
		KeyLabel:       "--label",
		KeyAnnotation:  "--annotation",
	}
	warn = lookupAndAddKeyVals(container, ContainerGroup, keyValKeys, podman)
	warnings = errors.Join(warnings, warn)

	masks := container.LookupAllArgs(ContainerGroup, KeyMask)
	for _, mask := range masks {
		podman.add("--security-opt", fmt.Sprintf("mask=%s", mask))
	}

	unmasks := container.LookupAllArgs(ContainerGroup, KeyUnmask)
	for _, unmask := range unmasks {
		podman.add("--security-opt", fmt.Sprintf("unmask=%s", unmask))
	}

	envFiles := container.LookupAllArgs(ContainerGroup, KeyEnvironmentFile)
	for _, envFile := range envFiles {
		filePath, err := getAbsolutePath(container, envFile)
		if err != nil {
			return nil, warnings, err
		}
		podman.add("--env-file", filePath)
	}

	secrets := container.LookupAllArgs(ContainerGroup, KeySecret)
	for _, secret := range secrets {
		podman.add("--secret", secret)
	}

	mounts := container.LookupAllArgs(ContainerGroup, KeyMount)
	for _, mount := range mounts {
		mountStr, err := resolveContainerMountParams(container, service, mount, unitsInfoMap)
		if err != nil {
			return nil, warnings, err
		}
		podman.add("--mount", mountStr)
	}

	handleHealth(container, ContainerGroup, podman)

	if err := handlePod(container, service, ContainerGroup, unitsInfoMap, podman); err != nil {
		return nil, warnings, err
	}

	handlePodmanArgs(container, ContainerGroup, podman)

	if len(image) > 0 {
		podman.add(image)
	} else {
		podman.add("--rootfs", rootfs)
	}

	execArgs, ok, warn := container.LookupLastArgs(ContainerGroup, KeyExec)
	if ok {
		podman.add(execArgs...)
	}
	warnings = errors.Join(warnings, warn)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	return service, warnings, nil
}

// Determine if the given file name belongs to an uninstantiated template unit.
func IsTemplateUnitFileName(fileName string) bool {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return strings.HasSuffix(base, "@")
}

// Get the unresolved container name that may contain '%'.
func getContainerName(container *parser.UnitFile) string {
	containerName, ok := container.Lookup(ContainerGroup, KeyContainerName)
	if !ok || len(containerName) == 0 {
		// By default, We want to name the container by the service name.
		if strings.Contains(container.Filename, "@") {
			containerName = "systemd-%p_%i"
		} else {
			containerName = "systemd-%N"
		}
	}
	return containerName
}

// Get the unresolved resource name that may contain '%'.
func getResourceName(unit *parser.UnitFile, group, key string) string {
	resourceName, ok := unit.Lookup(group, key)
	if !ok || len(resourceName) == 0 {
		resourceName = removeExtension(unit.Filename, "systemd-", "")
		// By default, We want to name the resource by the service name.
		if strings.Contains(unit.Filename, "@") {
			resourceName = resourceName[:len(resourceName)-1] + "-%i"
		}
	}
	return resourceName
}

// Get the resolved container name that contains no '%'.
// Returns an empty string if not resolvable.
func GetContainerResourceName(container *parser.UnitFile) string {
	containerName := getContainerName(container)

	// XXX: only %N is handled.
	// it is difficult to properly implement specifiers handling without consulting systemd.
	resourceName := strings.ReplaceAll(containerName, "%N", GetContainerServiceName(container))

	if !strings.Contains(resourceName, "%") {
		return resourceName
	} else {
		return ""
	}
}

func defaultOneshotServiceGroup(service *parser.UnitFile, remainAfterExit bool) {
	// The default syslog identifier is the exec basename (podman) which isn't very useful here
	if _, ok := service.Lookup(ServiceGroup, "SyslogIdentifier"); !ok {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}
	if _, ok := service.Lookup(ServiceGroup, "Type"); !ok {
		service.Set(ServiceGroup, "Type", "oneshot")
	}
	if remainAfterExit {
		if _, ok := service.Lookup(ServiceGroup, "RemainAfterExit"); !ok {
			service.Set(ServiceGroup, "RemainAfterExit", "yes")
		}
	}
}

// Convert a quadlet network file (unit file with a Network group) to a systemd
// service file (unit file with Service group) based on the options in the
// Network group.
// The original Network group is kept around as X-Network.
// Also returns the canonical network name, either auto-generated or user-defined via the
// NetworkName key-value.
func ConvertNetwork(network *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error, error) {
	var warn, warnings error

	service, unitInfo, err := initServiceUnitFile(network, isUser, unitsInfoMap, NetworkGroup)
	if err != nil {
		return nil, warnings, err
	}

	// Derive network name from unit name (with added prefix), or use user-provided name.
	networkName := getResourceName(network, NetworkGroup, KeyNetworkName)

	if network.LookupBooleanWithDefault(NetworkGroup, KeyNetworkDeleteOnStop, false) {
		serviceStopPostCmd := createBasePodmanCommand(network, NetworkGroup)
		serviceStopPostCmd.add("network", "rm", networkName)
		service.AddCmdline(ServiceGroup, "ExecStopPost", serviceStopPostCmd.Args)
	}

	podman := createBasePodmanCommand(network, NetworkGroup)

	podman.add("network", "create", "--ignore")

	boolKeys := map[string]string{
		KeyDisableDNS: "--disable-dns",
		KeyInternal:   "--internal",
		KeyIPv6:       "--ipv6",
	}
	lookupAndAddBoolean(network, NetworkGroup, boolKeys, podman)

	stringKeys := map[string]string{
		KeyDriver:        "--driver",
		KeyIPAMDriver:    "--ipam-driver",
		KeyInterfaceName: "--interface-name",
	}
	lookupAndAddString(network, NetworkGroup, stringKeys, podman)

	allStringKeys := map[string]string{
		KeyDNS: "--dns",
	}
	lookupAndAddAllStrings(network, NetworkGroup, allStringKeys, podman)

	subnets := network.LookupAll(NetworkGroup, KeySubnet)
	gateways := network.LookupAll(NetworkGroup, KeyGateway)
	ipRanges := network.LookupAll(NetworkGroup, KeyIPRange)
	if len(subnets) > 0 {
		if len(gateways) > len(subnets) {
			return nil, warnings, fmt.Errorf("cannot set more gateways than subnets")
		}
		if len(ipRanges) > len(subnets) {
			return nil, warnings, fmt.Errorf("cannot set more ranges than subnets")
		}
		for i := range subnets {
			podman.add("--subnet", subnets[i])
			if len(gateways) > i {
				podman.add("--gateway", gateways[i])
			}
			if len(ipRanges) > i {
				podman.add("--ip-range", ipRanges[i])
			}
		}
	} else if len(ipRanges) > 0 || len(gateways) > 0 {
		return nil, warnings, fmt.Errorf("cannot set gateway or range without subnet")
	}

	keyValKeys := map[string]string{
		KeyOptions: "--opt",
		KeyLabel:   "--label",
	}
	warn = lookupAndAddKeyVals(network, NetworkGroup, keyValKeys, podman)
	warnings = errors.Join(warnings, warn)

	handlePodmanArgs(network, NetworkGroup, podman)

	podman.add(networkName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	defaultOneshotServiceGroup(service, true)

	// Store the name of the created resource
	unitInfo.ResourceName = networkName
	return service, warnings, nil
}

// Convert a quadlet volume file (unit file with a Volume group) to a systemd
// service file (unit file with Service group) based on the options in the
// Volume group.
// The original Volume group is kept around as X-Volume.
// Also returns the canonical volume name, either auto-generated or user-defined via the VolumeName
// key-value.
func ConvertVolume(volume *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error, error) {
	var warn, warnings error

	service, unitInfo, err := initServiceUnitFile(volume, isUser, unitsInfoMap, VolumeGroup)
	if err != nil {
		return nil, warnings, err
	}

	// Derive volume name from unit name (with added prefix), or use user-provided name.
	volumeName := getResourceName(volume, VolumeGroup, KeyVolumeName)

	podman := createBasePodmanCommand(volume, VolumeGroup)

	podman.add("volume", "create", "--ignore")

	uid, ok := volume.Lookup(VolumeGroup, KeyUID)
	if ok {
		podman.add("--uid", uid)
	}

	gid, ok := volume.Lookup(VolumeGroup, KeyGID)
	if ok {
		podman.add("--gid", gid)
	}

	driver, ok := volume.Lookup(VolumeGroup, KeyDriver)
	if ok {
		podman.add("--driver", driver)
	}

	var opts strings.Builder

	if driver == "image" {
		opts.WriteString("image=")

		imageName, ok := volume.Lookup(VolumeGroup, KeyImage)
		if !ok {
			return nil, warnings, fmt.Errorf("the key %s is mandatory when using the image driver", KeyImage)
		}
		imageName, err := handleImageSource(imageName, service, unitsInfoMap)
		if err != nil {
			return nil, warnings, err
		}

		opts.WriteString(imageName)
	} else {
		opts.WriteString("o=")

		if volume.HasKey(VolumeGroup, "User") {
			uid := volume.LookupUint32(VolumeGroup, "User", 0)
			if opts.Len() > 2 {
				opts.WriteString(",")
			}
			fmt.Fprintf(&opts, "uid=%d", uid)
		}

		if volume.HasKey(VolumeGroup, "Group") {
			gid := volume.LookupUint32(VolumeGroup, "Group", 0)
			if opts.Len() > 2 {
				opts.WriteString(",")
			}
			fmt.Fprintf(&opts, "gid=%d", gid)
		}

		copy, ok := volume.LookupBoolean(VolumeGroup, KeyCopy)
		if ok {
			if copy {
				podman.add("--opt", "copy")
			} else {
				podman.add("--opt", "nocopy")
			}
		}

		devValid := false

		dev, ok := volume.Lookup(VolumeGroup, KeyDevice)
		if ok && len(dev) != 0 {
			podman.add("--opt", fmt.Sprintf("device=%s", dev))
			devValid = true
		}

		devType, ok := volume.Lookup(VolumeGroup, KeyType)
		if ok && len(devType) != 0 {
			if devValid {
				podman.add("--opt", fmt.Sprintf("type=%s", devType))
				if devType == "bind" {
					service.AddEscaped(UnitGroup, "RequiresMountsFor", dev)
				}
			} else {
				return nil, warnings, fmt.Errorf("key Type can't be used without Device")
			}
		}

		mountOpts, ok := volume.Lookup(VolumeGroup, KeyOptions)
		if ok && len(mountOpts) != 0 {
			if opts.Len() > 2 {
				opts.WriteString(",")
			}
			opts.WriteString(mountOpts)
		}
	}

	if opts.Len() > 2 {
		podman.add("--opt", opts.String())
	}

	keyValKeys := map[string]string{
		KeyLabel: "--label",
	}
	warn = lookupAndAddKeyVals(volume, VolumeGroup, keyValKeys, podman)
	warnings = errors.Join(warnings, warn)

	handlePodmanArgs(volume, VolumeGroup, podman)

	podman.add(volumeName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	defaultOneshotServiceGroup(service, true)

	// Store the name of the created resource
	unitInfo.ResourceName = volumeName

	return service, warnings, nil
}

func ConvertKube(kube *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error) {
	service, _, err := initServiceUnitFile(kube, isUser, unitsInfoMap, KubeGroup)
	if err != nil {
		return nil, err
	}

	yamlPaths := kube.LookupAllStrv(KubeGroup, KeyYaml)
	if len(yamlPaths) == 0 {
		return nil, fmt.Errorf("no Yaml key specified")
	}

	// Convert all yaml paths to absolute paths
	absoluteYamlPaths := make([]string, 0, len(yamlPaths))
	for _, yamlPath := range yamlPaths {
		absPath, err := getAbsolutePath(kube, yamlPath)
		if err != nil {
			return nil, err
		}
		absoluteYamlPaths = append(absoluteYamlPaths, absPath)
	}

	// Only allow mixed or control-group, as nothing else works well
	killMode, ok := service.Lookup(ServiceGroup, "KillMode")
	if !ok || (killMode != "mixed" && killMode != "control-group") {
		if ok {
			return nil, fmt.Errorf("invalid KillMode '%s'", killMode)
		}

		// We default to mixed instead of control-group, because it lets conmon do its thing
		service.Set(ServiceGroup, "KillMode", "mixed")
	}

	// Set PODMAN_SYSTEMD_UNIT so that podman auto-update can restart the service.
	service.Add(ServiceGroup, "Environment", "PODMAN_SYSTEMD_UNIT=%n")

	// Allow users to set the Service Type to oneshot to allow resources only kube yaml
	serviceType, ok := service.Lookup(ServiceGroup, "Type")
	if ok && serviceType != "notify" && serviceType != "oneshot" {
		return nil, fmt.Errorf("invalid service Type '%s'", serviceType)
	}

	if serviceType != "oneshot" {
		service.Setv(ServiceGroup,
			"Type", "notify",
			"NotifyAccess", "all")
	}

	if !kube.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	execStart := createBasePodmanCommand(kube, KubeGroup)

	execStart.add("kube", "play")

	execStart.add(
		// Replace any previous container with the same name, not fail
		"--replace",

		// Use a service container
		"--service-container=true",
	)

	if ecp, ok := kube.Lookup(KubeGroup, KeyExitCodePropagation); ok && len(ecp) > 0 {
		execStart.addf("--service-exit-code-propagation=%s", ecp)
	}

	handleLogDriver(kube, KubeGroup, execStart)
	handleLogOpt(kube, KubeGroup, execStart)

	if err := handleUserMappings(kube, KubeGroup, execStart, false); err != nil {
		return nil, err
	}

	if err := addNetworks(kube, KubeGroup, service, unitsInfoMap, execStart); err != nil {
		return nil, err
	}

	updateMaps := kube.LookupAllStrv(KubeGroup, KeyAutoUpdate)
	for _, update := range updateMaps {
		annotation := fmt.Sprintf("--annotation=%s", autoUpdateLabel)
		updateType := update
		annoValue, typ, hasSlash := strings.Cut(update, "/")
		if hasSlash {
			annotation = annotation + "/" + annoValue
			updateType = typ
		}
		execStart.addf("%s=%s", annotation, updateType)
	}

	configMaps := kube.LookupAllStrv(KubeGroup, KeyConfigMap)
	for _, configMap := range configMaps {
		configMapPath, err := getAbsolutePath(kube, configMap)
		if err != nil {
			return nil, err
		}
		execStart.add("--configmap", configMapPath)
	}

	handlePublishPorts(kube, KubeGroup, execStart)

	handlePodmanArgs(kube, KubeGroup, execStart)

	// Add all YAML file paths to the command
	execStart.add(absoluteYamlPaths...)

	service.AddCmdline(ServiceGroup, "ExecStart", execStart.Args)

	// Use `ExecStopPost` to make sure cleanup happens even in case of
	// errors; otherwise containers, pods, etc. would be left behind.
	execStop := createBasePodmanCommand(kube, KubeGroup)

	execStop.add("kube", "down")

	if kubeDownForce, ok := kube.LookupBoolean(KubeGroup, KeyKubeDownForce); ok {
		execStop.addBool("--force", kubeDownForce)
	}

	// Add all YAML file paths to the stop command
	execStop.add(absoluteYamlPaths...)
	service.AddCmdline(ServiceGroup, "ExecStopPost", execStop.Args)

	_, err = handleSetWorkingDirectory(kube, service, KubeGroup)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func ConvertImage(image *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error) {
	service, unitInfo, err := initServiceUnitFile(image, isUser, unitsInfoMap, ImageGroup)
	if err != nil {
		return nil, err
	}

	imageName, ok := image.Lookup(ImageGroup, KeyImage)
	if !ok || len(imageName) == 0 {
		return nil, fmt.Errorf("no Image key specified")
	}

	podman := createBasePodmanCommand(image, ImageGroup)

	podman.add("image", "pull")

	stringKeys := map[string]string{
		KeyArch:          "--arch",
		KeyAuthFile:      "--authfile",
		KeyCertDir:       "--cert-dir",
		KeyCreds:         "--creds",
		KeyDecryptionKey: "--decryption-key",
		KeyOS:            "--os",
		KeyPolicy:        "--policy",
		KeyVariant:       "--variant",
		KeyRetry:         "--retry",
		KeyRetryDelay:    "--retry-delay",
	}
	lookupAndAddString(image, ImageGroup, stringKeys, podman)

	boolKeys := map[string]string{
		KeyAllTags:   "--all-tags",
		KeyTLSVerify: "--tls-verify",
	}
	lookupAndAddBoolean(image, ImageGroup, boolKeys, podman)

	handlePodmanArgs(image, ImageGroup, podman)

	podman.add(imageName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	defaultOneshotServiceGroup(service, true)

	if name, ok := image.Lookup(ImageGroup, KeyImageTag); ok && len(name) > 0 {
		imageName = name
	}

	// Store the name of the created resource
	unitInfo.ResourceName = imageName

	return service, nil
}

func ConvertBuild(build *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error, error) {
	var warn, warnings error

	service, unitInfo, err := initServiceUnitFile(build, isUser, unitsInfoMap, BuildGroup)
	if err != nil {
		return nil, warnings, err
	}

	// Fast fail is ResouceName is not set
	if len(unitInfo.ResourceName) == 0 {
		return nil, warnings, fmt.Errorf("no ImageTag key specified")
	}

	podman := createBasePodmanCommand(build, BuildGroup)
	podman.add("build")

	// The `--pull` flag has to be handled separately and the `=` sign must be present
	// See https://github.com/containers/podman/issues/24599 for details
	if val, ok := build.Lookup(BuildGroup, KeyPull); ok && len(val) > 0 {
		podman.addf("--pull=%s", val)
	}

	stringKeys := map[string]string{
		KeyArch:       "--arch",
		KeyAuthFile:   "--authfile",
		KeyIgnoreFile: "--ignorefile",
		KeyTarget:     "--target",
		KeyVariant:    "--variant",
		KeyRetry:      "--retry",
		KeyRetryDelay: "--retry-delay",
	}
	lookupAndAddString(build, BuildGroup, stringKeys, podman)

	boolKeys := map[string]string{
		KeyTLSVerify: "--tls-verify",
		KeyForceRM:   "--force-rm",
	}
	lookupAndAddBoolean(build, BuildGroup, boolKeys, podman)

	allStringKeys := map[string]string{
		KeyDNS:       "--dns",
		KeyDNSOption: "--dns-option",
		KeyDNSSearch: "--dns-search",
		KeyGroupAdd:  "--group-add",
		KeyImageTag:  "--tag",
	}
	lookupAndAddAllStrings(build, BuildGroup, allStringKeys, podman)

	keyValKeys := map[string]string{
		KeyBuildArg:    "--build-arg",
		KeyEnvironment: "--env",
		KeyLabel:       "--label",
		KeyAnnotation:  "--annotation",
	}
	warn = lookupAndAddKeyVals(build, BuildGroup, keyValKeys, podman)
	warnings = errors.Join(warnings, warn)

	if err := addNetworks(build, BuildGroup, service, unitsInfoMap, podman); err != nil {
		return nil, warnings, err
	}

	secrets := build.LookupAllArgs(BuildGroup, KeySecret)
	for _, secret := range secrets {
		podman.add("--secret", secret)
	}

	if err := addVolumes(build, service, BuildGroup, unitsInfoMap, podman); err != nil {
		return nil, warnings, err
	}

	// In order to build an image locally, we need either a File key pointing directly at a
	// Containerfile, or we need a context or WorkingDirectory containing all required files.
	// SetWorkingDirectory= can also be a path, a URL to either a Containerfile, a Git repo, or
	// an archive.
	context, err := handleSetWorkingDirectory(build, service, BuildGroup)
	if err != nil {
		return nil, warnings, err
	}

	workingDirectory, okWD := service.Lookup(ServiceGroup, ServiceKeyWorkingDirectory)
	filePath, okFile := build.Lookup(BuildGroup, KeyFile)
	if (!okWD || len(workingDirectory) == 0) && (!okFile || len(filePath) == 0) && len(context) == 0 {
		return nil, warnings, fmt.Errorf("neither SetWorkingDirectory, nor File key specified")
	}

	if len(filePath) > 0 {
		podman.add("--file", filePath)
	}

	handlePodmanArgs(build, BuildGroup, podman)

	// Context or WorkingDirectory has to be last argument
	if len(context) > 0 {
		podman.add(context)
	} else if !startsWithSystemdSpecifier(filePath) && !filepath.IsAbs(filePath) && !isURL(filePath) {
		// Special handling for relative filePaths
		if len(workingDirectory) == 0 {
			return nil, warnings, fmt.Errorf("relative path in File key requires SetWorkingDirectory key to be set")
		}
		podman.add(workingDirectory)
	}

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	defaultOneshotServiceGroup(service, false)
	return service, warnings, nil
}

func GetBuiltImageName(buildUnit *parser.UnitFile) string {
	imageTags := buildUnit.LookupAll(BuildGroup, KeyImageTag)
	if len(imageTags) > 0 {
		return imageTags[0]
	}
	return ""
}

func GetUnitServiceName(unit *parser.UnitFile) (string, error) {
	switch {
	case strings.HasSuffix(unit.Filename, ".container"):
		return GetContainerServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".volume"):
		return GetVolumeServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".kube"):
		return GetKubeServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".network"):
		return GetNetworkServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".image"):
		return GetImageServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".build"):
		return GetBuildServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".artifact"):
		return GetArtifactServiceName(unit), nil
	case strings.HasSuffix(unit.Filename, ".pod"):
		return GetPodServiceName(unit), nil
	default:
		return "", fmt.Errorf("unsupported file type %q", unit.Filename)
	}
}

func GetContainerServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, ContainerGroup, "")
}

func GetKubeServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, KubeGroup, "")
}

func GetVolumeServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, VolumeGroup, "-volume")
}

func GetNetworkServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, NetworkGroup, "-network")
}

func GetImageServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, ImageGroup, "-image")
}

func GetBuildServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, BuildGroup, "-build")
}

func GetArtifactServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, ArtifactGroup, "-artifact")
}

func GetPodServiceName(podUnit *parser.UnitFile) string {
	return getServiceName(podUnit, PodGroup, "-pod")
}

func getServiceName(quadletUnitFile *parser.UnitFile, groupName string, defaultExtraSuffix string) string {
	if serviceName, ok := quadletUnitFile.Lookup(groupName, KeyServiceName); ok {
		return serviceName
	}
	baseServiceName := removeExtension(quadletUnitFile.Filename, "", "")
	if baseServiceName[len(baseServiceName)-1] == '@' {
		baseServiceName = baseServiceName[:len(baseServiceName)-1]
		defaultExtraSuffix += "@"
	}
	return baseServiceName + defaultExtraSuffix
}

func GetPodResourceName(podUnit *parser.UnitFile) string {
	// Derive pod name from unit name (with added prefix), or use user-provided name.
	podName, ok := podUnit.Lookup(PodGroup, KeyPodName)
	if !ok || len(podName) == 0 {
		podName = removeExtension(podUnit.Filename, "systemd-", "")
	}
	return podName
}

func ConvertPod(podUnit *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error, error) {
	var warn, warnings error

	service, unitInfo, err := initServiceUnitFile(podUnit, isUser, unitsInfoMap, PodGroup)
	if err != nil {
		return nil, warnings, err
	}

	podName := GetPodResourceName(podUnit)

	for _, containerService := range unitInfo.ContainersToStart {
		service.Add(UnitGroup, "Wants", containerService)
		service.Add(UnitGroup, "Before", containerService)
	}

	if !podUnit.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	execStart := createBasePodmanCommand(podUnit, PodGroup)
	execStart.add("pod", "start", podName)
	service.AddCmdline(ServiceGroup, "ExecStart", execStart.Args)

	execStop := createBasePodmanCommand(podUnit, PodGroup)
	execStop.add("pod", "stop")
	stopTimeout := "10"
	if value, ok := podUnit.Lookup(PodGroup, KeyStopTimeout); ok {
		stopTimeout = value
	}
	execStop.add(
		"--ignore",
		fmt.Sprintf("--time=%s", stopTimeout),
		podName,
	)
	service.AddCmdline(ServiceGroup, "ExecStop", execStop.Args)

	execStopPost := createBasePodmanCommand(podUnit, PodGroup)
	execStopPost.add("pod", "rm")
	execStopPost.add(
		"--ignore",
		"--force",
		podName,
	)
	service.AddCmdline(ServiceGroup, "ExecStopPost", execStopPost.Args)

	execStartPre := createBasePodmanCommand(podUnit, PodGroup)
	execStartPre.add("pod", "create")
	execStartPre.add(
		"--infra-conmon-pidfile=%t/%N.pid",
		"--replace",
	)

	handleExitPolicy(podUnit, PodGroup, execStartPre)

	if err := handleUserMappings(podUnit, PodGroup, execStartPre, true); err != nil {
		return nil, warnings, err
	}

	handlePublishPorts(podUnit, PodGroup, execStartPre)

	keyValKeys := map[string]string{
		KeyLabel: "--label",
	}
	warn = lookupAndAddKeyVals(podUnit, PodGroup, keyValKeys, execStartPre)
	warnings = errors.Join(warnings, warn)

	if err := addNetworks(podUnit, PodGroup, service, unitsInfoMap, execStartPre); err != nil {
		return nil, warnings, err
	}

	stringsKeys := map[string]string{
		KeyIP:      "--ip",
		KeyIP6:     "--ip6",
		KeyShmSize: "--shm-size",
	}
	lookupAndAddString(podUnit, PodGroup, stringsKeys, execStartPre)

	allStringsKeys := map[string]string{
		KeyNetworkAlias: "--network-alias",
		KeyDNS:          "--dns",
		KeyDNSOption:    "--dns-option",
		KeyDNSSearch:    "--dns-search",
		KeyAddHost:      "--add-host",
		KeyHostName:     "--hostname",
	}
	lookupAndAddAllStrings(podUnit, PodGroup, allStringsKeys, execStartPre)

	if err := addVolumes(podUnit, service, PodGroup, unitsInfoMap, execStartPre); err != nil {
		return nil, warnings, err
	}

	execStartPre.add("--infra-name", fmt.Sprintf("%s-infra", podName))
	execStartPre.add("--name", podName)

	handlePodmanArgs(podUnit, PodGroup, execStartPre)

	service.AddCmdline(ServiceGroup, "ExecStartPre", execStartPre.Args)

	// Set PODMAN_SYSTEMD_UNIT so that podman auto-update can restart the service.
	service.Add(ServiceGroup, "Environment", "PODMAN_SYSTEMD_UNIT=%n")

	service.Setv(ServiceGroup,
		"Type", "forking",
		"PIDFile", "%t/%N.pid",
	)

	if !podUnit.HasKey(ServiceGroup, "Restart") {
		service.Set(ServiceGroup, "Restart", "on-failure")
	}

	return service, warnings, nil
}

func handleUser(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) error {
	user, hasUser := unitFile.Lookup(groupName, KeyUser)
	okUser := hasUser && len(user) > 0

	group, hasGroup := unitFile.Lookup(groupName, KeyGroup)
	okGroup := hasGroup && len(group) > 0

	if !okUser {
		if okGroup {
			return fmt.Errorf("invalid Group set without User")
		}
		return nil
	}

	var userGroupStr string
	if !okGroup {
		userGroupStr = user
	} else {
		userGroupStr = fmt.Sprintf("%s:%s", user, group)
	}
	podman.add("--user", userGroupStr)

	return nil
}

func handleUserMappings(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline, supportManual bool) error {
	mappingsDefined := false

	if userns, ok := unitFile.Lookup(groupName, KeyUserNS); ok && len(userns) > 0 {
		podman.add("--userns", userns)
		mappingsDefined = true
	}

	uidMaps := unitFile.LookupAllStrv(groupName, KeyUIDMap)
	mappingsDefined = mappingsDefined || len(uidMaps) > 0
	for _, uidMap := range uidMaps {
		podman.add("--uidmap", uidMap)
	}

	gidMaps := unitFile.LookupAllStrv(groupName, KeyGIDMap)
	mappingsDefined = mappingsDefined || len(gidMaps) > 0
	for _, gidMap := range gidMaps {
		podman.add("--gidmap", gidMap)
	}

	if subUIDMap, ok := unitFile.Lookup(groupName, KeySubUIDMap); ok && len(subUIDMap) > 0 {
		podman.add("--subuidname", subUIDMap)
		mappingsDefined = true
	}

	if subGIDMap, ok := unitFile.Lookup(groupName, KeySubGIDMap); ok && len(subGIDMap) > 0 {
		podman.add("--subgidname", subGIDMap)
		mappingsDefined = true
	}

	if mappingsDefined {
		_, hasRemapUID := unitFile.Lookup(groupName, KeyRemapUid)
		_, hasRemapGID := unitFile.Lookup(groupName, KeyRemapGid)
		_, RemapUsers := unitFile.LookupLast(groupName, KeyRemapUsers)
		if hasRemapUID || hasRemapGID || RemapUsers {
			return fmt.Errorf("deprecated Remap keys are set along with explicit mapping keys")
		}
		return nil
	}

	return handleUserRemap(unitFile, groupName, podman, supportManual)
}

func handleUserRemap(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline, supportManual bool) error {
	uidMaps := unitFile.LookupAllStrv(groupName, KeyRemapUid)
	gidMaps := unitFile.LookupAllStrv(groupName, KeyRemapGid)
	remapUsers, _ := unitFile.LookupLast(groupName, KeyRemapUsers)
	switch remapUsers {
	case "":
		if len(uidMaps) > 0 {
			return fmt.Errorf("UidMap set without RemapUsers")
		}
		if len(gidMaps) > 0 {
			return fmt.Errorf("GidMap set without RemapUsers")
		}
	case "manual":
		if supportManual {
			for _, uidMap := range uidMaps {
				podman.add("--uidmap", uidMap)
			}
			for _, gidMap := range gidMaps {
				podman.add("--gidmap", gidMap)
			}
		} else {
			return fmt.Errorf("RemapUsers=manual is not supported")
		}
	case "auto":
		autoOpts := make([]string, 0)
		for _, uidMap := range uidMaps {
			autoOpts = append(autoOpts, "uidmapping="+uidMap)
		}
		for _, gidMap := range gidMaps {
			autoOpts = append(autoOpts, "gidmapping="+gidMap)
		}
		uidSize := unitFile.LookupUint32(groupName, KeyRemapUidSize, 0)
		if uidSize > 0 {
			autoOpts = append(autoOpts, fmt.Sprintf("size=%v", uidSize))
		}

		podman.add("--userns", usernsOpts("auto", autoOpts))
	case "keep-id":
		keepidOpts := make([]string, 0)
		if len(uidMaps) > 0 {
			if len(uidMaps) > 1 {
				return fmt.Errorf("RemapUsers=keep-id supports only a single value for UID mapping")
			}
			keepidOpts = append(keepidOpts, "uid="+uidMaps[0])
		}
		if len(gidMaps) > 0 {
			if len(gidMaps) > 1 {
				return fmt.Errorf("RemapUsers=keep-id supports only a single value for GID mapping")
			}
			keepidOpts = append(keepidOpts, "gid="+gidMaps[0])
		}

		podman.add("--userns", usernsOpts("keep-id", keepidOpts))

	default:
		return fmt.Errorf("unsupported RemapUsers option '%s'", remapUsers)
	}

	return nil
}

func addNetworks(quadletUnitFile *parser.UnitFile, groupName string, serviceUnitFile *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, podman *PodmanCmdline) error {
	networks := quadletUnitFile.LookupAll(groupName, KeyNetwork)
	for _, network := range networks {
		if len(network) > 0 {
			quadletNetworkName, options, found := strings.Cut(network, ":")

			isNetworkUnit := strings.HasSuffix(quadletNetworkName, ".network")
			isContainerUnit := strings.HasSuffix(quadletNetworkName, ".container")

			if isNetworkUnit || isContainerUnit {
				unitInfo, ok := unitsInfoMap[quadletNetworkName]
				if !ok {
					return fmt.Errorf("requested Quadlet unit %s was not found", quadletNetworkName)
				}

				// XXX: this is usually because a '@' in service name
				if len(unitInfo.ResourceName) == 0 {
					return fmt.Errorf("cannot get the resource name of %s", quadletNetworkName)
				}

				// the systemd unit name is $serviceName.service
				serviceFileName := unitInfo.ServiceFileName()

				serviceUnitFile.Add(UnitGroup, "Requires", serviceFileName)
				serviceUnitFile.Add(UnitGroup, "After", serviceFileName)

				if found {
					if isContainerUnit {
						return fmt.Errorf("extra options are not supported when joining another container's network")
					}
					network = fmt.Sprintf("%s:%s", unitInfo.ResourceName, options)
				} else {
					if isContainerUnit {
						network = fmt.Sprintf("container:%s", unitInfo.ResourceName)
					} else {
						network = unitInfo.ResourceName
					}
				}
			}

			podman.add("--network", network)
		}
	}
	return nil
}

// Systemd Specifiers start with % with the exception of %%
func startsWithSystemdSpecifier(filePath string) bool {
	if len(filePath) == 0 || filePath[0] != '%' {
		return false
	}

	if len(filePath) > 1 && filePath[1] == '%' {
		return false
	}

	return true
}

func getAbsolutePath(quadletUnitFile *parser.UnitFile, filePath string) (string, error) {
	// When the path starts with a Systemd specifier do not resolve what looks like a relative address
	if !startsWithSystemdSpecifier(filePath) && !filepath.IsAbs(filePath) {
		if len(quadletUnitFile.Path) > 0 {
			filePath = filepath.Join(filepath.Dir(quadletUnitFile.Path), filePath)
		} else {
			var err error
			filePath, err = filepath.Abs(filePath)
			if err != nil {
				return "", err
			}
		}
	}
	return filePath, nil
}

func handleExitPolicy(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	exitPolicy, found := unitFile.Lookup(groupName, KeyExitPolicy)

	podman.add("--exit-policy")
	if found {
		podman.add(exitPolicy)
	} else {
		podman.add("stop")
	}
}

func handlePublishPorts(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	publishPorts := unitFile.LookupAll(groupName, KeyPublishPort)
	for _, publishPort := range publishPorts {
		podman.add("--publish", publishPort)
	}
}

func handleLogDriver(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	logDriver, found := unitFile.Lookup(groupName, KeyLogDriver)
	if found {
		podman.add("--log-driver", logDriver)
	}
}

func handleLogOpt(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	logOpts := unitFile.LookupAllStrv(groupName, KeyLogOpt)
	for _, logOpt := range logOpts {
		podman.add("--log-opt", logOpt)
	}
}

func handleStorageSource(quadletUnitFile, serviceUnitFile *parser.UnitFile, source string, unitsInfoMap map[string]*UnitInfo, checkImage bool) (string, error) {
	if source == "" {
		return "", fmt.Errorf("source cannot be empty")
	}
	if source[0] == '.' {
		var err error
		source, err = getAbsolutePath(quadletUnitFile, source)
		if err != nil {
			return "", err
		}
	}
	if source[0] == '/' {
		// Absolute path
		serviceUnitFile.AddEscaped(UnitGroup, "RequiresMountsFor", source)
	} else if strings.HasSuffix(source, ".volume") || (checkImage && strings.HasSuffix(source, ".image")) || strings.HasSuffix(source, ".artifact") {
		sourceUnitInfo, ok := unitsInfoMap[source]
		if !ok {
			return "", fmt.Errorf("requested Quadlet source %s was not found", source)
		}

		// the systemd unit name is $serviceName.service
		sourceServiceName := sourceUnitInfo.ServiceFileName()
		serviceUnitFile.Add(UnitGroup, "Requires", sourceServiceName)
		serviceUnitFile.Add(UnitGroup, "After", sourceServiceName)

		source = sourceUnitInfo.ResourceName
	}

	return source, nil
}

func handleHealth(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	keyArgMap := [][2]string{
		{KeyHealthCmd, "cmd"},
		{KeyHealthInterval, "interval"},
		{KeyHealthOnFailure, "on-failure"},
		{KeyHealthLogDestination, "log-destination"},
		{KeyHealthMaxLogCount, "max-log-count"},
		{KeyHealthMaxLogSize, "max-log-size"},
		{KeyHealthRetries, "retries"},
		{KeyHealthStartPeriod, "start-period"},
		{KeyHealthTimeout, "timeout"},
		{KeyHealthStartupCmd, "startup-cmd"},
		{KeyHealthStartupInterval, "startup-interval"},
		{KeyHealthStartupRetries, "startup-retries"},
		{KeyHealthStartupSuccess, "startup-success"},
		{KeyHealthStartupTimeout, "startup-timeout"},
	}

	for _, keyArg := range keyArgMap {
		val, found := unitFile.Lookup(groupName, keyArg[0])
		if found && len(val) > 0 {
			podman.addf("--health-%s", keyArg[1])
			podman.addf("%s", val)
		}
	}
}

func handlePodmanArgs(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	podmanArgs := unitFile.LookupAllArgs(groupName, KeyPodmanArgs)
	if len(podmanArgs) > 0 {
		podman.add(podmanArgs...)
	}
}

func handleSetWorkingDirectory(quadletUnitFile, serviceUnitFile *parser.UnitFile, quadletGroup string) (string, error) {
	setWorkingDirectory, ok := quadletUnitFile.Lookup(quadletGroup, KeySetWorkingDirectory)
	if !ok || len(setWorkingDirectory) == 0 {
		return "", nil
	}

	var relativeToFile string
	var context string
	switch strings.ToLower(setWorkingDirectory) {
	case "yaml":
		if quadletGroup != KubeGroup {
			return "", fmt.Errorf("SetWorkingDirectory=%s is only supported in .kube files", setWorkingDirectory)
		}

		yamlPaths := quadletUnitFile.LookupAllStrv(KubeGroup, KeyYaml)
		if len(yamlPaths) == 0 {
			return "", fmt.Errorf("no Yaml key specified")
		} else if len(yamlPaths) != 1 {
			return "", fmt.Errorf("SetWorkingDirectory=yaml is only supported when a single Yaml key is provided")
		}

		relativeToFile = yamlPaths[0]
	case "file":
		if quadletGroup != BuildGroup {
			return "", fmt.Errorf("SetWorkingDirectory=%s is only supported in .build files", setWorkingDirectory)
		}

		relativeToFile, ok = quadletUnitFile.Lookup(quadletGroup, KeyFile)
		if !ok {
			return "", fmt.Errorf("no File key specified")
		}
	case "unit":
		relativeToFile = quadletUnitFile.Path
	default:
		// Path / URL handling is for .build files only
		if quadletGroup != BuildGroup {
			return "", fmt.Errorf("unsupported value for %s: %s ", ServiceKeyWorkingDirectory, setWorkingDirectory)
		}

		// Any value other than the above cases will be returned as context
		context = setWorkingDirectory

		// If we have a relative path, set the WorkingDirectory to that of the
		// quadletUnitFile
		if !filepath.IsAbs(context) {
			relativeToFile = quadletUnitFile.Path
		}
	}

	if len(relativeToFile) > 0 && !isURL(context) {
		// If WorkingDirectory is already set in the Service section do not change it
		workingDir, ok := quadletUnitFile.Lookup(ServiceGroup, ServiceKeyWorkingDirectory)
		if ok && len(workingDir) > 0 {
			return "", nil
		}

		fileInWorkingDir, err := getAbsolutePath(quadletUnitFile, relativeToFile)
		if err != nil {
			return "", err
		}

		serviceUnitFile.Add(ServiceGroup, ServiceKeyWorkingDirectory, filepath.Dir(fileInWorkingDir))
	}

	return context, nil
}

func lookupAndAddString(unit *parser.UnitFile, group string, keys map[string]string, podman *PodmanCmdline) {
	for key, flag := range keys {
		if val, ok := unit.Lookup(group, key); ok && len(val) > 0 {
			podman.add(flag, val)
		}
	}
}

func lookupAndAddAllStrings(unit *parser.UnitFile, group string, keys map[string]string, podman *PodmanCmdline) {
	for key, flag := range keys {
		values := unit.LookupAll(group, key)
		for _, val := range values {
			podman.add(flag, val)
		}
	}
}

func lookupAndAddBoolean(unit *parser.UnitFile, group string, keys map[string]string, podman *PodmanCmdline) {
	for key, flag := range keys {
		if val, ok := unit.LookupBoolean(group, key); ok {
			podman.addBool(flag, val)
		}
	}
}

func handleImageSource(quadletImageName string, serviceUnitFile *parser.UnitFile, unitsInfoMap map[string]*UnitInfo) (string, error) {
	for _, suffix := range []string{".build", ".image"} {
		if strings.HasSuffix(quadletImageName, suffix) {
			// since there is no default name conversion, the actual image name must exist in the names map
			unitInfo, ok := unitsInfoMap[quadletImageName]
			if !ok {
				return "", fmt.Errorf("requested Quadlet image %s was not found", quadletImageName)
			}

			// the systemd unit name is $name-$suffix.service
			imageServiceName := unitInfo.ServiceFileName()

			serviceUnitFile.Add(UnitGroup, "Requires", imageServiceName)
			serviceUnitFile.Add(UnitGroup, "After", imageServiceName)

			quadletImageName = unitInfo.ResourceName
		}
	}

	return quadletImageName, nil
}

func resolveContainerMountParams(containerUnitFile, serviceUnitFile *parser.UnitFile, mount string, unitsInfoMap map[string]*UnitInfo) (string, error) {
	mountType, tokens, err := specgenutilexternal.FindMountType(mount)
	if err != nil {
		return "", err
	}

	// Source resolution is required only for these types of mounts
	sourceResultionRequired := map[string]struct{}{
		"volume":   {},
		"bind":     {},
		"glob":     {},
		"image":    {},
		"artifact": {},
	}
	if _, ok := sourceResultionRequired[mountType]; !ok {
		return mount, nil
	}

	sourceIndex := -1
	originalSource := ""
	for i, token := range tokens {
		key, val, hasVal := strings.Cut(token, "=")
		if key == "source" || key == "src" {
			if !hasVal {
				return "", fmt.Errorf("source parameter does not include a value")
			}
			sourceIndex = i
			originalSource = val
		}
	}

	if originalSource != "" {
		resolvedSource, err := handleStorageSource(containerUnitFile, serviceUnitFile, originalSource, unitsInfoMap, true)
		if err != nil {
			return "", err
		}
		tokens[sourceIndex] = fmt.Sprintf("source=%s", resolvedSource)
	} else if mountType != "volume" {
		return "", fmt.Errorf("source parameter is required for mount type %s", mountType)
	}

	tokens = append([]string{fmt.Sprintf("type=%s", mountType)}, tokens...)

	return convertToCSV(tokens)
}

func convertToCSV(s []string) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	err := writer.Write(s)
	if err != nil {
		return "", err
	}
	writer.Flush()

	ret := buf.String()
	if ret[len(ret)-1] == '\n' {
		ret = ret[:len(ret)-1]
	}

	return ret, nil
}

func createBasePodmanCommand(unitFile *parser.UnitFile, groupName string) *PodmanCmdline {
	podman := NewPodmanCmdline()

	containersConfModules := unitFile.LookupAll(groupName, KeyContainersConfModule)
	for _, containersConfModule := range containersConfModules {
		podman.addf("--module=%s", containersConfModule)
	}

	globalArgs := unitFile.LookupAllArgs(groupName, KeyGlobalArgs)
	if len(globalArgs) > 0 {
		podman.add(globalArgs...)
	}

	return podman
}

func handlePod(quadletUnitFile, serviceUnitFile *parser.UnitFile, groupName string, unitsInfoMap map[string]*UnitInfo, podman *PodmanCmdline) error {
	pod, ok := quadletUnitFile.Lookup(groupName, KeyPod)
	if ok && len(pod) > 0 {
		if !strings.HasSuffix(pod, ".pod") {
			return fmt.Errorf("pod %s is not Quadlet based", pod)
		}

		podInfo, ok := unitsInfoMap[pod]
		if !ok {
			return fmt.Errorf("quadlet pod unit %s does not exist", pod)
		}

		podman.add("--pod", podInfo.ResourceName)

		podServiceName := podInfo.ServiceFileName()
		serviceUnitFile.Add(UnitGroup, "BindsTo", podServiceName)
		serviceUnitFile.Add(UnitGroup, "After", podServiceName)

		// If we want to start the container with the pod, we add it to this list.
		// This creates corresponding Wants=/Before= statements in the pod service.
		// Do not add this for template units as dependency cannot be created for them.
		if !IsTemplateUnitFileName(quadletUnitFile.Filename) && quadletUnitFile.LookupBooleanWithDefault(groupName, KeyStartWithPod, true) {
			podInfo.ContainersToStart = append(podInfo.ContainersToStart, serviceUnitFile.Filename)
		}
	}
	return nil
}

func addVolumes(quadletUnitFile, serviceUnitFile *parser.UnitFile, groupName string, unitsInfoMap map[string]*UnitInfo, podman *PodmanCmdline) error {
	volumes := quadletUnitFile.LookupAll(groupName, KeyVolume)
	for _, volume := range volumes {
		parts := strings.SplitN(volume, ":", 3)

		source := ""
		var dest string
		options := ""
		if len(parts) >= 2 {
			source = parts[0]
			dest = parts[1]
		} else {
			dest = parts[0]
		}
		if len(parts) >= 3 {
			options = ":" + parts[2]
		}

		if source != "" {
			var err error
			source, err = handleStorageSource(quadletUnitFile, serviceUnitFile, source, unitsInfoMap, false)
			if err != nil {
				return err
			}
		}

		podman.add("-v")
		if source == "" {
			podman.add(dest)
		} else {
			podman.addf("%s:%s%s", source, dest, options)
		}
	}

	return nil
}

func addDefaultDependencies(service *parser.UnitFile, isUser bool) {
	// Add a dependency on network-online.target so the image pull container startup
	// does not happen before network is ready.
	// https://github.com/containers/podman/issues/21873
	if service.LookupBooleanWithDefault(QuadletGroup, KeyDefaultDependencies, true) {
		networkUnit := "network-online.target"
		// network-online.target only exists as root and user session cannot wait for it
		// https://github.com/systemd/systemd/issues/3312
		// Given this is a bad problem with pasta which can fail to start or use the
		// wrong interface if the network is not fully set up we need to work around
		// that: https://github.com/containers/podman/issues/22197.
		if isUser {
			networkUnit = "podman-user-wait-network-online.service"
		}
		service.PrependUnitLine(UnitGroup, "After", networkUnit)
		service.PrependUnitLine(UnitGroup, "Wants", networkUnit)
	}
}

// handleExecReload handles the ExecReload key.
// If return (warning, error)
// An error is returned if both KeyReloadCmd and KeyReloadSignal are set,
// and a warning is returned if it failed to parse the ReloadCmd key.
func handleExecReload(quadletUnitFile, serviceUnitFile *parser.UnitFile, groupName, containerName string) (error, error) {
	reloadSignal, signalOk := quadletUnitFile.Lookup(groupName, KeyReloadSignal)
	signalOk = signalOk && len(reloadSignal) > 0
	reloadcmd, cmdOk, warn := quadletUnitFile.LookupLastArgs(groupName, KeyReloadCmd)
	cmdOk = cmdOk && len(reloadcmd) > 0

	if !cmdOk && !signalOk {
		return warn, nil
	}

	if cmdOk && signalOk {
		return warn, fmt.Errorf("%s and %s are mutually exclusive but both are set", KeyReloadCmd, KeyReloadSignal)
	}

	serviceReloadCmd := createBasePodmanCommand(quadletUnitFile, groupName)
	if cmdOk {
		serviceReloadCmd.add("exec", containerName)
		serviceReloadCmd.add(reloadcmd...)
	} else {
		serviceReloadCmd.add("kill", "--signal", reloadSignal, containerName)
	}
	serviceUnitFile.AddCmdline(ServiceGroup, "ExecReload", serviceReloadCmd.Args)

	return warn, nil
}

func translateUnitDependencies(serviceUnitFile *parser.UnitFile, unitsInfoMap map[string]*UnitInfo) error {
	for _, unitDependencyKey := range unitDependencyKeys {
		deps := serviceUnitFile.LookupAllStrv(UnitGroup, unitDependencyKey)
		if len(deps) == 0 {
			continue
		}
		translatedDeps := make([]string, 0, len(deps))
		translated := false
		for _, dep := range deps {
			var translatedDep string

			ext := filepath.Ext(dep)
			if _, ok := SupportedExtensions[ext]; ok {
				unitInfo, ok := unitsInfoMap[dep]
				if !ok {
					return fmt.Errorf("unable to translate dependency for %s", dep)
				}
				translatedDep = unitInfo.ServiceFileName()
				translated = true
			} else {
				translatedDep = dep
			}
			translatedDeps = append(translatedDeps, translatedDep)
		}
		if !translated {
			continue
		}
		serviceUnitFile.Unset(UnitGroup, unitDependencyKey)
		serviceUnitFile.Add(UnitGroup, unitDependencyKey, strings.Join(translatedDeps, " "))
	}
	return nil
}

func lookupAndAddKeyVals(unit *parser.UnitFile, group string, keys map[string]string, podman *PodmanCmdline) error {
	var warnings error
	for key, flag := range keys {
		keyVals, warn := unit.LookupAllKeyVal(group, key)
		warnings = errors.Join(warnings, warn)
		podman.addKeys(flag, keyVals)
	}
	return warnings
}

func initServiceUnitFile(quadletUnitFile *parser.UnitFile, isUser bool, unitsInfoMap map[string]*UnitInfo, group string) (*parser.UnitFile, *UnitInfo, error) {
	unitInfo, ok := unitsInfoMap[quadletUnitFile.Filename]
	if !ok {
		return nil, nil, fmt.Errorf("internal error while processing container %s", quadletUnitFile.Filename)
	}

	if err := checkForUnknownKeys(quadletUnitFile, group, groupsInfo[group].SupportedKeys); err != nil {
		return nil, nil, err
	}

	service := quadletUnitFile.Dup()
	service.Filename = unitInfo.ServiceFileName()

	if err := translateUnitDependencies(service, unitsInfoMap); err != nil {
		return nil, nil, err
	}

	addDefaultDependencies(service, isUser)

	if quadletUnitFile.Path != "" {
		service.Add(UnitGroup, "SourcePath", quadletUnitFile.Path)
	}

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	// Rename old Container group to x-Container so that systemd ignores it
	service.RenameGroup(group, groupsInfo[group].XGroupName)

	// Rename common quadlet group
	service.RenameGroup(QuadletGroup, XQuadletGroup)

	return service, unitInfo, nil
}

func ConvertArtifact(artifact *parser.UnitFile, unitsInfoMap map[string]*UnitInfo, isUser bool) (*parser.UnitFile, error) {
	service, unitInfo, err := initServiceUnitFile(artifact, isUser, unitsInfoMap, ArtifactGroup)
	if err != nil {
		return nil, err
	}

	artifactName, ok := artifact.Lookup(ArtifactGroup, KeyArtifact)
	if !ok || len(artifactName) == 0 {
		return nil, fmt.Errorf("no Artifact key specified")
	}

	podman := createBasePodmanCommand(artifact, ArtifactGroup)

	podman.add("artifact", "pull")

	stringKeys := map[string]string{
		KeyAuthFile:      "--authfile",
		KeyCertDir:       "--cert-dir",
		KeyCreds:         "--creds",
		KeyDecryptionKey: "--decryption-key",
		KeyRetry:         "--retry",
		KeyRetryDelay:    "--retry-delay",
	}
	lookupAndAddString(artifact, ArtifactGroup, stringKeys, podman)

	boolKeys := map[string]string{
		KeyQuiet:     "--quiet",
		KeyTLSVerify: "--tls-verify",
	}
	lookupAndAddBoolean(artifact, ArtifactGroup, boolKeys, podman)

	handlePodmanArgs(artifact, ArtifactGroup, podman)

	podman.add(artifactName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	defaultOneshotServiceGroup(service, true)

	unitInfo.ResourceName = artifactName

	return service, nil
}
