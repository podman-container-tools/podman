package common

import (
	"errors"
	"time"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	commonFlag "go.podman.io/common/pkg/flag"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

const sizeWithUnitFormat = "(format: `<number>[<unit>]`, where unit = b (bytes), k (kibibytes), m (mebibytes), or g (gibibytes))"

var podmanConfig = registry.PodmanConfig()

// ContainerToPodOptions takes the Container and Pod Create options, assigning the matching values back to podCreate for the purpose of the libpod API
// For this function to succeed, the JSON tags in PodCreateOptions and ContainerCreateOptions need to match due to the Marshalling and Unmarshalling done.
// The types of the options also need to match or else the unmarshalling will fail even if the tags match
func ContainerToPodOptions(containerCreate *entities.ContainerCreateOptions, podCreate *entities.PodCreateOptions) error {
	contMarshal, err := json.Marshal(containerCreate)
	if err != nil {
		return err
	}
	return json.Unmarshal(contMarshal, podCreate)
}

// DefineCreateFlags declares and instantiates the container create flags
func DefineCreateFlags(cmd *cobra.Command, cf *entities.ContainerCreateOptions, mode entities.ContainerMode) {
	createFlags := cmd.Flags()

	if mode == entities.CreateMode { // regular create flags
		annotationFlagName := "annotation"
		createFlags.StringArrayVar(
			&cf.Annotation,
			annotationFlagName, []string{},
			"Add annotations to container (key=value)",
		)
		_ = cmd.RegisterFlagCompletionFunc(annotationFlagName, completion.AutocompleteNone)

		attachFlagName := "attach"
		createFlags.StringSliceVarP(
			&cf.Attach,
			attachFlagName, "a", []string{},
			"Attach to STDIN, STDOUT or STDERR",
		)
		_ = cmd.RegisterFlagCompletionFunc(attachFlagName, AutocompleteCreateAttach)

		authfileFlagName := "authfile"
		createFlags.StringVar(
			&cf.Authfile,
			authfileFlagName, auth.GetDefaultAuthFile(),
			"Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override",
		)
		_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

		capAddFlagName := "cap-add"
		createFlags.StringSliceVar(
			&cf.CapAdd,
			capAddFlagName, []string{},
			"Add capabilities to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(capAddFlagName, completion.AutocompleteCapabilities)

		capDropFlagName := "cap-drop"
		createFlags.StringSliceVar(
			&cf.CapDrop,
			capDropFlagName, []string{},
			"Drop capabilities from the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(capDropFlagName, completion.AutocompleteCapabilities)

		cgroupnsFlagName := "cgroupns"
		createFlags.String(
			cgroupnsFlagName, "",
			"cgroup namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupnsFlagName, AutocompleteNamespace)

		cgroupsFlagName := "cgroups"
		createFlags.StringVar(
			&cf.CgroupsMode,
			cgroupsFlagName, cf.CgroupsMode,
			`control container cgroup configuration ("enabled"|"disabled"|"no-conmon"|"split")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupsFlagName, AutocompleteCgroupMode)

		cidfileFlagName := "cidfile"
		createFlags.StringVar(
			&cf.CIDFile,
			cidfileFlagName, "",
			"Write the container ID to the file",
		)
		_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

		deviceCgroupRuleFlagName := "device-cgroup-rule"
		createFlags.StringSliceVar(
			&cf.DeviceCgroupRule,
			deviceCgroupRuleFlagName, []string{},
			"Add a rule to the cgroup allowed devices list",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceCgroupRuleFlagName, completion.AutocompleteNone)

		createFlags.Bool(
			"disable-content-trust", false,
			"This is a Docker specific option and is a NOOP",
		)

		envMergeFlagName := "env-merge"
		createFlags.StringArrayVar(
			&cf.EnvMerge,
			envMergeFlagName, []string{},
			"Preprocess environment variables from image before injecting them into the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(envMergeFlagName, completion.AutocompleteNone)

		createFlags.BoolVar(
			&cf.UnsetEnvAll,
			"unsetenv-all", false,
			"Unset all default environment variables in container",
		)

		if !registry.IsRemote() {
			createFlags.BoolVar(
				&cf.EnvHost,
				"env-host",
				podmanConfig.ContainersConfDefaultsRO.Containers.EnvHost,
				"Use all current host environment variables in container",
			)
		}

		envFileFlagName := "env-file"
		createFlags.StringArrayVar(
			&cf.EnvFile,
			envFileFlagName, []string{},
			"Read in a file of environment variables",
		)
		_ = cmd.RegisterFlagCompletionFunc(envFileFlagName, completion.AutocompleteDefault)

		exposeFlagName := "expose"
		createFlags.StringSliceVar(
			&cf.Expose,
			exposeFlagName, []string{},
			"Expose a port or a range of ports",
		)
		_ = cmd.RegisterFlagCompletionFunc(exposeFlagName, completion.AutocompleteNone)

		groupAddFlagName := "group-add"
		createFlags.StringSliceVar(
			&cf.GroupAdd,
			groupAddFlagName, []string{},
			"Add additional groups to the primary container process. 'keep-groups' allows container processes to use supplementary groups.",
		)
		_ = cmd.RegisterFlagCompletionFunc(groupAddFlagName, completion.AutocompleteNone)

		createFlags.BoolVar(
			&cf.HTTPProxy,
			"http-proxy", podmanConfig.ContainersConfDefaultsRO.Containers.HTTPProxy,
			"Set proxy environment variables in the container based on the host proxy vars",
		)

		hostUserFlagName := "hostuser"
		createFlags.StringSliceVar(
			&cf.HostUsers,
			hostUserFlagName, []string{},
			"Host user account to add to /etc/passwd within container",
		)
		_ = cmd.RegisterFlagCompletionFunc(hostUserFlagName, completion.AutocompleteNone)

		imageVolumeFlagName := "image-volume"
		createFlags.String(
			imageVolumeFlagName, cf.ImageVolume,
			`Tells podman how to handle the builtin image volumes ("bind"|"tmpfs"|"ignore")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(imageVolumeFlagName, AutocompleteImageVolume)

		createFlags.BoolVar(
			&cf.Init,
			"init", false,
			"Run an init binary inside the container that forwards signals and reaps processes",
		)

		initPathFlagName := "init-path"
		createFlags.StringVar(
			&cf.InitPath,
			initPathFlagName, "",
			// Do not use  the Value field for setting the default value to determine user input (i.e., non-empty string)
			"Path to the container-init binary",
		)
		_ = cmd.RegisterFlagCompletionFunc(initPathFlagName, completion.AutocompleteDefault)

		createFlags.BoolVarP(
			&cf.Interactive,
			"interactive", "i", false,
			"Make STDIN available to the contained process",
		)
		ipcFlagName := "ipc"
		createFlags.String(
			ipcFlagName, "",
			"IPC namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(ipcFlagName, AutocompleteNamespace)

		createFlags.String(
			"kernel-memory", "",
			"DEPRECATED: Option is just hear for compatibility with Docker",
		)
		// kernel-memory is deprecated in the runtime spec.
		_ = createFlags.MarkHidden("kernel-memory")

		logDriverFlagName := "log-driver"
		createFlags.StringVar(
			&cf.LogDriver,
			logDriverFlagName, cf.LogDriver,
			"Logging driver for the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(logDriverFlagName, AutocompleteLogDriver)

		logOptFlagName := "log-opt"
		createFlags.StringArrayVar(
			&cf.LogOptions,
			logOptFlagName, []string{},
			"Logging driver options",
		)
		_ = cmd.RegisterFlagCompletionFunc(logOptFlagName, AutocompleteLogOpt)

		createFlags.BoolVar(
			&cf.OOMKillDisable,
			"oom-kill-disable", false,
			"Disable OOM Killer",
		)

		oomScoreAdjFlagName := "oom-score-adj"
		createFlags.Int(
			oomScoreAdjFlagName, 0,
			"Tune the host's OOM preferences (-1000 to 1000)",
		)
		_ = cmd.RegisterFlagCompletionFunc(oomScoreAdjFlagName, completion.AutocompleteNone)

		archFlagName := "arch"
		createFlags.StringVar(
			&cf.Arch,
			archFlagName, "",
			"use `ARCH` instead of the architecture of the machine for choosing images",
		)
		_ = cmd.RegisterFlagCompletionFunc(archFlagName, completion.AutocompleteArch)

		osFlagName := "os"
		createFlags.StringVar(
			&cf.OS,
			osFlagName, "",
			"use `OS` instead of the running OS for choosing images",
		)
		_ = cmd.RegisterFlagCompletionFunc(osFlagName, completion.AutocompleteOS)

		variantFlagName := "variant"
		createFlags.StringVar(
			&cf.Variant,
			variantFlagName, "",
			"Use `VARIANT` instead of the running architecture variant for choosing images",
		)
		_ = cmd.RegisterFlagCompletionFunc(variantFlagName, completion.AutocompleteNone)

		platformFlagName := "platform"
		createFlags.StringVar(
			&cf.Platform,
			platformFlagName, "",
			"Specify the platform for selecting the image.  (Conflicts with --arch and --os)",
		)
		_ = cmd.RegisterFlagCompletionFunc(platformFlagName, completion.AutocompleteNone)

		podIDFileFlagName := "pod-id-file"
		createFlags.StringVar(
			&cf.PodIDFile,
			podIDFileFlagName, "",
			"Read the pod ID from the file",
		)
		_ = cmd.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)
		createFlags.BoolVar(
			&cf.Privileged,
			"privileged", podmanConfig.ContainersConfDefaultsRO.Containers.Privileged,
			"Give extended privileges to container",
		)
		createFlags.BoolVarP(
			&cf.PublishAll,
			"publish-all", "P", false,
			"Publish all exposed ports to random ports on the host interface",
		)

		pullFlagName := "pull"
		createFlags.StringVar(
			&cf.Pull,
			pullFlagName, cf.Pull,
			`Pull image policy ("always"|"missing"|"never"|"newer")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(pullFlagName, AutocompletePullOption)

		createFlags.BoolVarP(
			&cf.Quiet,
			"quiet", "q", false,
			"Suppress output information when pulling images",
		)
		rdtClassFlagName := "rdt-class"
		createFlags.StringVar(
			&cf.IntelRdtClosID,
			rdtClassFlagName, cf.IntelRdtClosID,
			"Class of Service (COS) that the container should be assigned to",
		)
		_ = cmd.RegisterFlagCompletionFunc(rdtClassFlagName, AutocompletePullOption)

		createFlags.BoolVar(
			&cf.ReadOnly,
			"read-only", podmanConfig.ContainersConfDefaultsRO.Containers.ReadOnly,
			"Make containers root filesystem read-only",
		)
		createFlags.BoolVar(
			&cf.ReadWriteTmpFS,
			"read-only-tmpfs", cf.ReadWriteTmpFS,
			"When running --read-only containers mount read-write tmpfs on /dev, /dev/shm, /run, /tmp and /var/tmp",
		)
		requiresFlagName := "requires"
		createFlags.StringSliceVar(
			&cf.Requires,
			requiresFlagName, []string{},
			"Add one or more requirement containers that must be started before this container will start",
		)
		_ = cmd.RegisterFlagCompletionFunc(requiresFlagName, AutocompleteContainers)

		retryFlagName := "retry"
		createFlags.Uint(retryFlagName, registry.RetryDefault(), "number of times to retry in case of failure when performing pull")
		_ = cmd.RegisterFlagCompletionFunc(retryFlagName, completion.AutocompleteNone)
		retryDelayFlagName := "retry-delay"
		createFlags.String(retryDelayFlagName, registry.RetryDelayDefault(), "delay between retries in case of pull failures")
		_ = cmd.RegisterFlagCompletionFunc(retryDelayFlagName, completion.AutocompleteNone)

		createFlags.BoolVar(
			&cf.Rm,
			"rm", false,
			"Remove container and any anonymous unnamed volume associated with the container after exit",
		)
		createFlags.BoolVar(
			&cf.RootFS,
			"rootfs", false,
			"The first argument is not an image but the rootfs to the exploded container",
		)

		sdnotifyFlagName := "sdnotify"
		createFlags.StringVar(
			&cf.SdNotifyMode,
			sdnotifyFlagName, cf.SdNotifyMode,
			`control sd-notify behavior ("container"|"conmon"|"healthy"|"ignore")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(sdnotifyFlagName, AutocompleteSDNotify)

		secretFlagName := "secret"
		createFlags.StringArrayVar(
			&cf.Secrets,
			secretFlagName, []string{},
			"Add secret to container",
		)
		_ = cmd.RegisterFlagCompletionFunc(secretFlagName, AutocompleteSecrets)

		stopSignalFlagName := "stop-signal"
		createFlags.StringVar(
			&cf.StopSignal,
			stopSignalFlagName, "",
			"Signal to stop a container. Default is SIGTERM",
		)
		_ = cmd.RegisterFlagCompletionFunc(stopSignalFlagName, AutocompleteStopSignal)

		stopTimeoutFlagName := "stop-timeout"
		createFlags.UintVar(
			&cf.StopTimeout,
			stopTimeoutFlagName, cf.StopTimeout,
			"Timeout (in seconds) that containers stopped by user command have to exit. If exceeded, the container will be forcibly stopped via SIGKILL.",
		)
		_ = cmd.RegisterFlagCompletionFunc(stopTimeoutFlagName, completion.AutocompleteNone)

		systemdFlagName := "systemd"
		createFlags.StringVar(
			&cf.Systemd,
			systemdFlagName, cf.Systemd,
			`Run container in systemd mode ("true"|"false"|"always")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(systemdFlagName, AutocompleteSystemdFlag)

		personalityFlagName := "personality"
		createFlags.StringVar(
			&cf.Personality,
			personalityFlagName, "",
			"Configure execution domain using personality (e.g., LINUX/LINUX32)",
		)
		_ = cmd.RegisterFlagCompletionFunc(personalityFlagName, AutocompleteNamespace)

		timeoutFlagName := "timeout"
		createFlags.UintVar(
			&cf.Timeout,
			timeoutFlagName, 0,
			"Maximum length of time a container is allowed to run. The container will be killed automatically after the time expires.",
		)
		_ = cmd.RegisterFlagCompletionFunc(timeoutFlagName, completion.AutocompleteNone)

		commonFlag.OptionalBoolFlag(createFlags,
			&cf.TLSVerify,
			"tls-verify",
			"Require HTTPS and verify certificates when contacting registries for pulling images",
		)

		tmpfsFlagName := "tmpfs"
		createFlags.StringArrayVar(
			&cf.TmpFS,
			tmpfsFlagName, []string{},
			"Mount a temporary filesystem (`tmpfs`) into a container",
		)
		_ = cmd.RegisterFlagCompletionFunc(tmpfsFlagName, completion.AutocompleteDefault)

		createFlags.BoolVarP(
			&cf.TTY,
			"tty", "t", false,
			"Allocate a pseudo-TTY for container",
		)

		timezoneFlagName := "tz"
		createFlags.StringVar(
			&cf.Timezone,
			timezoneFlagName, cf.Timezone,
			"Set timezone in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(timezoneFlagName, completion.AutocompleteNone) // TODO: add timezone completion

		umaskFlagName := "umask"
		createFlags.StringVar(
			&cf.Umask,
			umaskFlagName, cf.Umask,
			"Set umask in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(umaskFlagName, completion.AutocompleteNone)

		userFlagName := "user"
		createFlags.StringVarP(
			&cf.User,
			userFlagName, "u", "",
			"Username or UID (format: <name|uid>[:<group|gid>])",
		)
		_ = cmd.RegisterFlagCompletionFunc(userFlagName, AutocompleteUserFlag)

		mountFlagName := "mount"
		createFlags.StringArrayVar(
			&cf.Mount,
			mountFlagName, []string{},
			"Attach a filesystem mount to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(mountFlagName, AutocompleteMountFlag)

		workdirFlagName := "workdir"
		createFlags.StringVarP(
			&cf.Workdir,
			workdirFlagName, "w", "",
			"Working directory inside the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(workdirFlagName, completion.AutocompleteDefault)

		seccompPolicyFlagName := "seccomp-policy"
		createFlags.StringVar(
			&cf.SeccompPolicy,
			seccompPolicyFlagName, cf.SeccompPolicy,
			"Policy for selecting a seccomp profile (experimental)",
		)
		_ = cmd.RegisterFlagCompletionFunc(seccompPolicyFlagName, completion.AutocompleteDefault)

		cgroupConfFlagName := "cgroup-conf"
		createFlags.StringSliceVar(
			&cf.CgroupConf,
			cgroupConfFlagName, []string{},
			"Configure cgroup v2 (key=value)",
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupConfFlagName, completion.AutocompleteNone)

		pidFileFlagName := "pidfile"
		createFlags.StringVar(
			&cf.PidFile,
			pidFileFlagName, "",
			"Write the container process ID to the file")
		_ = cmd.RegisterFlagCompletionFunc(pidFileFlagName, completion.AutocompleteDefault)

		chrootDirsFlagName := "chrootdirs"
		createFlags.StringArrayVar(
			&cf.ChrootDirs,
			chrootDirsFlagName, []string{},
			"Chroot directories inside the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(chrootDirsFlagName, completion.AutocompleteDefault)

		passwdEntryName := "passwd-entry"
		createFlags.StringVar(&cf.PasswdEntry, passwdEntryName, "", "Entry to write to /etc/passwd")
		_ = cmd.RegisterFlagCompletionFunc(passwdEntryName, completion.AutocompleteNone)

		groupEntryName := "group-entry"
		createFlags.StringVar(&cf.GroupEntry, groupEntryName, "", "Entry to write to /etc/group")
		_ = cmd.RegisterFlagCompletionFunc(groupEntryName, completion.AutocompleteNone)

		decryptionKeysFlagName := "decryption-key"
		createFlags.StringArrayVar(
			&cf.DecryptionKeys,
			decryptionKeysFlagName, []string{},
			"Key needed to decrypt the image (e.g. /path/to/key.pem)",
		)
		_ = cmd.RegisterFlagCompletionFunc(decryptionKeysFlagName, completion.AutocompleteNone)

		if registry.IsRemote() {
			_ = createFlags.MarkHidden("env-host")
			_ = createFlags.MarkHidden(decryptionKeysFlagName)
		} else {
			createFlags.StringVar(
				&cf.SignaturePolicy,
				"signature-policy", "",
				"`Pathname` of signature policy file (not usually used)",
			)
			_ = createFlags.MarkHidden("signature-policy")

			certDirFlagName := "cert-dir"
			createFlags.StringVar(
				&cf.CertDir,
				certDirFlagName, "",
				"`Pathname` of a directory containing TLS certificates and keys",
			)
			_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)
		}

		credsFlagName := "creds"
		createFlags.StringVar(
			&cf.Creds,
			credsFlagName, "",
			"`credentials` (USERNAME:PASSWORD) to use for authenticating to a registry",
		)
		_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteDefault)

		createFlags.BoolVar(
			&cf.Replace,
			"replace", false,
			`If a container with the same name exists, replace it`,
		)
	}
	if mode == entities.CreateMode || mode == entities.UpdateMode {
		createFlags.BoolVar(
			&cf.NoHealthCheck,
			"no-healthcheck", false,
			"Disable healthchecks on container",
		)

		envFlagName := "env"
		createFlags.StringArrayP(
			envFlagName, "e", Env(),
			"Set environment variables in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(envFlagName, completion.AutocompleteNone)

		unsetenvFlagName := "unsetenv"
		createFlags.StringArrayVar(
			&cf.UnsetEnv,
			unsetenvFlagName, []string{},
			"Unset environment default variables in container",
		)
		_ = cmd.RegisterFlagCompletionFunc(unsetenvFlagName, completion.AutocompleteNone)

		ulimitFlagName := "ulimit"
		createFlags.StringSliceVar(
			&cf.Ulimit,
			ulimitFlagName, cf.Ulimit,
			"Ulimit options",
		)
		_ = cmd.RegisterFlagCompletionFunc(ulimitFlagName, completion.AutocompleteNone)

		healthCmdFlagName := "health-cmd"
		createFlags.StringVar(
			&cf.HealthCmd,
			healthCmdFlagName, "",
			"set a healthcheck command for the container ('none' disables the existing healthcheck)",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthCmdFlagName, completion.AutocompleteNone)

		info := ""
		if mode == entities.UpdateMode {
			info = "Changing this setting resets timer."
		}
		healthIntervalFlagName := "health-interval"
		createFlags.StringVar(
			&cf.HealthInterval,
			healthIntervalFlagName, define.DefaultHealthCheckInterval,
			"set an interval for the healthcheck. (a value of disable results in no automatic timer setup) "+info,
		)
		_ = cmd.RegisterFlagCompletionFunc(healthIntervalFlagName, completion.AutocompleteNone)

		warning := ""
		if mode == entities.UpdateMode {
			warning = "Warning: Changing this setting may cause the loss of previous logs!"
		}
		healthLogDestinationFlagName := "health-log-destination"
		createFlags.StringVar(
			&cf.HealthLogDestination,
			healthLogDestinationFlagName, define.DefaultHealthCheckLocalDestination,
			"set the destination of the HealthCheck log. Directory path, local or events_logger (local use container state file) "+warning,
		)
		_ = cmd.RegisterFlagCompletionFunc(healthLogDestinationFlagName, completion.AutocompleteNone)

		healthMaxLogCountFlagName := "health-max-log-count"
		createFlags.UintVar(
			&cf.HealthMaxLogCount,
			healthMaxLogCountFlagName, define.DefaultHealthMaxLogCount,
			"set maximum number of attempts in the HealthCheck log file. ('0' value means an infinite number of attempts in the log file)",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthMaxLogCountFlagName, completion.AutocompleteNone)

		healthMaxLogSizeFlagName := "health-max-log-size"
		createFlags.UintVar(
			&cf.HealthMaxLogSize,
			healthMaxLogSizeFlagName, define.DefaultHealthMaxLogSize,
			"set maximum length in characters of stored HealthCheck log. ('0' value means an infinite log length)",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthMaxLogSizeFlagName, completion.AutocompleteNone)

		healthRetriesFlagName := "health-retries"
		createFlags.UintVar(
			&cf.HealthRetries,
			healthRetriesFlagName, define.DefaultHealthCheckRetries,
			"the number of retries allowed before a healthcheck is considered to be unhealthy",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthRetriesFlagName, completion.AutocompleteNone)

		healthStartPeriodFlagName := "health-start-period"
		createFlags.StringVar(
			&cf.HealthStartPeriod,
			healthStartPeriodFlagName, define.DefaultHealthCheckStartPeriod,
			"the initialization time needed for a container to bootstrap",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthStartPeriodFlagName, completion.AutocompleteNone)

		healthTimeoutFlagName := "health-timeout"
		createFlags.StringVar(
			&cf.HealthTimeout,
			healthTimeoutFlagName, define.DefaultHealthCheckTimeout,
			"the maximum time allowed to complete the healthcheck before an interval is considered failed and SIGKILL is sent to the healthcheck process",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthTimeoutFlagName, completion.AutocompleteNone)

		healthOnFailureFlagName := "health-on-failure"
		createFlags.StringVar(
			&cf.HealthOnFailure,
			healthOnFailureFlagName, "none",
			"action to take once the container turns unhealthy",
		)
		_ = cmd.RegisterFlagCompletionFunc(healthOnFailureFlagName, AutocompleteHealthOnFailure)

		// Startup HealthCheck

		startupHCCmdFlagName := "health-startup-cmd"
		createFlags.StringVar(
			&cf.StartupHCCmd,
			startupHCCmdFlagName, "",
			"Set a startup healthcheck command for the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(startupHCCmdFlagName, completion.AutocompleteNone)

		info = ""
		if mode == entities.UpdateMode {
			info = "Changing this setting resets the timer, depending on the state of the container."
		}
		startupHCIntervalFlagName := "health-startup-interval"
		createFlags.StringVar(
			&cf.StartupHCInterval,
			startupHCIntervalFlagName, define.DefaultHealthCheckInterval,
			"Set an interval for the startup healthcheck. "+info,
		)
		_ = cmd.RegisterFlagCompletionFunc(startupHCIntervalFlagName, completion.AutocompleteNone)

		startupHCRetriesFlagName := "health-startup-retries"
		createFlags.UintVar(
			&cf.StartupHCRetries,
			startupHCRetriesFlagName, 0,
			"Set the maximum number of retries before the startup healthcheck will restart the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(startupHCRetriesFlagName, completion.AutocompleteNone)

		startupHCSuccessesFlagName := "health-startup-success"
		createFlags.UintVar(
			&cf.StartupHCSuccesses,
			startupHCSuccessesFlagName, 0,
			"Set the number of consecutive successes before the startup healthcheck is marked as successful and the normal healthcheck begins (0 indicates any success will start the regular healthcheck)",
		)
		_ = cmd.RegisterFlagCompletionFunc(startupHCSuccessesFlagName, completion.AutocompleteNone)

		startupHCTimeoutFlagName := "health-startup-timeout"
		createFlags.StringVar(
			&cf.StartupHCTimeout,
			startupHCTimeoutFlagName, define.DefaultHealthCheckTimeout,
			"Set the maximum amount of time that the startup healthcheck may take before it is considered failed",
		)
		_ = cmd.RegisterFlagCompletionFunc(startupHCTimeoutFlagName, completion.AutocompleteNone)
	}

	// Restart is allowed for created, updated, and infra ctr
	if mode == entities.InfraMode || mode == entities.CreateMode || mode == entities.UpdateMode {
		restartFlagName := "restart"
		createFlags.StringVar(
			&cf.Restart,
			restartFlagName, "",
			`Restart policy to apply when a container exits ("always"|"no"|"never"|"on-failure"|"unless-stopped")`,
		)
		_ = cmd.RegisterFlagCompletionFunc(restartFlagName, AutocompleteRestartOption)
	}
	if mode == entities.InfraMode || (mode == entities.CreateMode) { // infra container flags, create should also pick these up
		shmSizeFlagName := "shm-size"
		createFlags.String(
			shmSizeFlagName, shmSize(),
			"Size of /dev/shm "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(shmSizeFlagName, completion.AutocompleteNone)

		shmSizeSystemdFlagName := "shm-size-systemd"
		createFlags.String(
			shmSizeSystemdFlagName, "",
			"Size of systemd specific tmpfs mounts (/run, /run/lock) "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(shmSizeSystemdFlagName, completion.AutocompleteNone)

		sysctlFlagName := "sysctl"
		createFlags.StringSliceVar(
			&cf.Sysctl,
			sysctlFlagName, []string{},
			"Sysctl options",
		)

		_ = cmd.RegisterFlagCompletionFunc(sysctlFlagName, AutocompleteSysctl)

		securityOptFlagName := "security-opt"
		createFlags.StringArrayVar(
			&cf.SecurityOpt,
			securityOptFlagName, []string{},
			"Security Options",
		)
		_ = cmd.RegisterFlagCompletionFunc(securityOptFlagName, AutocompleteSecurityOption)

		subgidnameFlagName := "subgidname"
		createFlags.StringVar(
			&cf.SubUIDName,
			subgidnameFlagName, "",
			"Name of range listed in /etc/subgid for use in user namespace",
		)
		_ = cmd.RegisterFlagCompletionFunc(subgidnameFlagName, completion.AutocompleteSubgidName)

		subuidnameFlagName := "subuidname"
		createFlags.StringVar(
			&cf.SubGIDName,
			subuidnameFlagName, "",
			"Name of range listed in /etc/subuid for use in user namespace",
		)
		_ = cmd.RegisterFlagCompletionFunc(subuidnameFlagName, completion.AutocompleteSubuidName)

		gidmapFlagName := "gidmap"
		createFlags.StringSliceVar(
			&cf.GIDMap,
			gidmapFlagName, []string{},
			"GID map to use for the user namespace",
		)
		_ = cmd.RegisterFlagCompletionFunc(gidmapFlagName, completion.AutocompleteNone)

		gpuFlagName := "gpus"
		createFlags.StringSliceVar(&cf.GPUs, gpuFlagName, []string{}, "GPU devices to add to the container ('all' to pass all GPUs)")
		_ = cmd.RegisterFlagCompletionFunc(gpuFlagName, completion.AutocompleteNone)

		uidmapFlagName := "uidmap"
		createFlags.StringSliceVar(
			&cf.UIDMap,
			uidmapFlagName, []string{},
			"UID map to use for the user namespace",
		)
		_ = cmd.RegisterFlagCompletionFunc(uidmapFlagName, completion.AutocompleteNone)

		usernsFlagName := "userns"
		createFlags.String(
			usernsFlagName, "",
			"User namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(usernsFlagName, AutocompleteUserNamespace)

		utsFlagName := "uts"
		createFlags.StringVar(
			&cf.UTS,
			utsFlagName, "",
			"UTS namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(utsFlagName, AutocompleteNamespace)

		cgroupParentFlagName := "cgroup-parent"
		createFlags.StringVar(
			&cf.CgroupParent,
			cgroupParentFlagName, "",
			"Optional parent cgroup for the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(cgroupParentFlagName, completion.AutocompleteDefault)
		var conmonPidfileFlagName string
		if mode == entities.CreateMode {
			conmonPidfileFlagName = "conmon-pidfile"
		} else {
			conmonPidfileFlagName = "infra-conmon-pidfile"
		}
		createFlags.StringVar(
			&cf.ConmonPIDFile,
			conmonPidfileFlagName, "",
			"Path to the file that will receive the PID of conmon",
		)
		_ = cmd.RegisterFlagCompletionFunc(conmonPidfileFlagName, completion.AutocompleteDefault)

		var entrypointFlagName string
		if mode == entities.CreateMode {
			entrypointFlagName = "entrypoint"
		} else {
			entrypointFlagName = "infra-command"
		}

		createFlags.String(entrypointFlagName, "",
			"Overwrite the default ENTRYPOINT of the image",
		)
		_ = cmd.RegisterFlagCompletionFunc(entrypointFlagName, completion.AutocompleteNone)

		hostnameFlagName := "hostname"
		createFlags.StringVarP(
			&cf.Hostname,
			hostnameFlagName, "h", "",
			"Set container hostname",
		)
		_ = cmd.RegisterFlagCompletionFunc(hostnameFlagName, completion.AutocompleteNone)

		labelFlagName := "label"
		createFlags.StringArrayVarP(
			&cf.Label,
			labelFlagName, "l", []string{},
			"Set metadata on container",
		)
		_ = cmd.RegisterFlagCompletionFunc(labelFlagName, completion.AutocompleteNone)

		labelFileFlagName := "label-file"
		createFlags.StringArrayVar(
			&cf.LabelFile,
			labelFileFlagName, []string{},
			"Read in a line delimited file of labels",
		)
		_ = cmd.RegisterFlagCompletionFunc(labelFileFlagName, completion.AutocompleteDefault)

		if mode == entities.InfraMode {
			nameFlagName := "infra-name"
			createFlags.StringVar(
				&cf.Name,
				nameFlagName, "",
				"Assign a name to the container",
			)
			_ = cmd.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)
		}

		createFlags.Bool(
			"help", false, "",
		)

		pidFlagName := "pid"
		createFlags.StringVar(
			&cf.PID,
			pidFlagName, "",
			"PID namespace to use",
		)
		_ = cmd.RegisterFlagCompletionFunc(pidFlagName, AutocompleteNamespace)

		volumeDescription := "Bind mount a volume into the container"
		if registry.IsRemote() {
			volumeDescription = "Bind mount a volume into the container. Volume source will be on the server machine, not the client"
		}
		volumeFlagName := "volume"
		createFlags.StringArrayVarP(
			&cf.Volume,
			volumeFlagName, "v", cf.Volume,
			volumeDescription,
		)
		_ = cmd.RegisterFlagCompletionFunc(volumeFlagName, AutocompleteVolumeFlag)

		deviceFlagName := "device"
		createFlags.StringArrayVar(
			&cf.Devices,
			deviceFlagName, devices(),
			"Add a host device to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceFlagName, completion.AutocompleteDefault)

		volumesFromFlagName := "volumes-from"
		createFlags.StringArrayVar(
			&cf.VolumesFrom,
			volumesFromFlagName, []string{},
			"Mount volumes from the specified container(s)",
		)
		_ = cmd.RegisterFlagCompletionFunc(volumesFromFlagName, AutocompleteContainers)
	}

	if mode == entities.CloneMode || mode == entities.CreateMode {
		nameFlagName := "name"
		createFlags.StringVar(
			&cf.Name,
			nameFlagName, "",
			"Assign a name to the container",
		)
		_ = cmd.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)

		podFlagName := "pod"
		createFlags.StringVar(
			&cf.Pod,
			podFlagName, "",
			"Run container in an existing pod",
		)
		_ = cmd.RegisterFlagCompletionFunc(podFlagName, AutocompletePods)
	}
	if mode != entities.InfraMode { // clone create and update only flags, we need this level of separation so clone does not pick up all of the flags
		cpuPeriodFlagName := "cpu-period"
		createFlags.Uint64Var(
			&cf.CPUPeriod,
			cpuPeriodFlagName, 0,
			"Limit the CPU CFS (Completely Fair Scheduler) period",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuPeriodFlagName, completion.AutocompleteNone)

		cpuQuotaFlagName := "cpu-quota"
		createFlags.Int64Var(
			&cf.CPUQuota,
			cpuQuotaFlagName, 0,
			"Limit the CPU CFS (Completely Fair Scheduler) quota",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuQuotaFlagName, completion.AutocompleteNone)

		cpuRtPeriodFlagName := "cpu-rt-period"
		createFlags.Uint64Var(
			&cf.CPURTPeriod,
			cpuRtPeriodFlagName, 0,
			"Limit the CPU real-time period in microseconds",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuRtPeriodFlagName, completion.AutocompleteNone)

		cpuRtRuntimeFlagName := "cpu-rt-runtime"
		createFlags.Int64Var(
			&cf.CPURTRuntime,
			cpuRtRuntimeFlagName, 0,
			"Limit the CPU real-time runtime in microseconds",
		)
		_ = cmd.RegisterFlagCompletionFunc(cpuRtRuntimeFlagName, completion.AutocompleteNone)

		memoryReservationFlagName := "memory-reservation"
		createFlags.StringVar(
			&cf.MemoryReservation,
			memoryReservationFlagName, "",
			"Memory soft limit "+sizeWithUnitFormat,
		)
		_ = cmd.RegisterFlagCompletionFunc(memoryReservationFlagName, completion.AutocompleteNone)

		memorySwappinessFlagName := "memory-swappiness"
		createFlags.Int64Var(
			&cf.MemorySwappiness,
			memorySwappinessFlagName, cf.MemorySwappiness,
			"Tune container memory swappiness (0 to 100, or -1 for system default)",
		)
		_ = cmd.RegisterFlagCompletionFunc(memorySwappinessFlagName, completion.AutocompleteNone)
	}
	if mode == entities.CreateMode || mode == entities.UpdateMode {
		deviceReadIopsFlagName := "device-read-iops"
		createFlags.StringArrayVar(
			&cf.DeviceReadIOPs,
			deviceReadIopsFlagName, []string{},
			"Limit read rate (IO per second) from a device (e.g. --device-read-iops=/dev/sda:1000)",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceReadIopsFlagName, completion.AutocompleteDefault)

		deviceWriteIopsFlagName := "device-write-iops"
		createFlags.StringArrayVar(
			&cf.DeviceWriteIOPs,
			deviceWriteIopsFlagName, []string{},
			"Limit write rate (IO per second) to a device (e.g. --device-write-iops=/dev/sda:1000)",
		)
		_ = cmd.RegisterFlagCompletionFunc(deviceWriteIopsFlagName, completion.AutocompleteDefault)

		pidsLimitFlagName := "pids-limit"
		createFlags.Int64(
			pidsLimitFlagName, pidsLimit(),
			"Tune container pids limit (set -1 for unlimited)",
		)
		_ = cmd.RegisterFlagCompletionFunc(pidsLimitFlagName, completion.AutocompleteNone)
	}
	// anyone can use these
	cpusFlagName := "cpus"
	createFlags.Float64Var(
		&cf.CPUS,
		cpusFlagName, 0,
		"Number of CPUs. The default is 0.000 which means no limit",
	)
	_ = cmd.RegisterFlagCompletionFunc(cpusFlagName, completion.AutocompleteNone)

	cpusetCpusFlagName := "cpuset-cpus"
	createFlags.StringVar(
		&cf.CPUSetCPUs,
		cpusetCpusFlagName, "",
		"CPUs in which to allow execution (0-3, 0,1)",
	)
	_ = cmd.RegisterFlagCompletionFunc(cpusetCpusFlagName, completion.AutocompleteNone)

	memoryFlagName := "memory"
	createFlags.StringVarP(
		&cf.Memory,
		memoryFlagName, "m", "",
		"Memory limit "+sizeWithUnitFormat,
	)
	_ = cmd.RegisterFlagCompletionFunc(memoryFlagName, completion.AutocompleteNone)

	cpuSharesFlagName := "cpu-shares"
	createFlags.Uint64VarP(
		&cf.CPUShares,
		cpuSharesFlagName, "c", 0,
		"CPU shares (relative weight)",
	)
	_ = cmd.RegisterFlagCompletionFunc(cpuSharesFlagName, completion.AutocompleteNone)

	cpusetMemsFlagName := "cpuset-mems"
	createFlags.StringVar(
		&cf.CPUSetMems,
		cpusetMemsFlagName, "",
		"Memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.",
	)
	_ = cmd.RegisterFlagCompletionFunc(cpusetMemsFlagName, completion.AutocompleteNone)

	memorySwapFlagName := "memory-swap"
	createFlags.StringVar(
		&cf.MemorySwap,
		memorySwapFlagName, "",
		"Swap limit equal to memory plus swap: '-1' to enable unlimited swap",
	)
	_ = cmd.RegisterFlagCompletionFunc(memorySwapFlagName, completion.AutocompleteNone)

	deviceReadBpsFlagName := "device-read-bps"
	createFlags.StringArrayVar(
		&cf.DeviceReadBPs,
		deviceReadBpsFlagName, []string{},
		"Limit read rate (bytes per second) from a device (e.g. --device-read-bps=/dev/sda:1mb)",
	)
	_ = cmd.RegisterFlagCompletionFunc(deviceReadBpsFlagName, completion.AutocompleteDefault)

	deviceWriteBpsFlagName := "device-write-bps"
	createFlags.StringArrayVar(
		&cf.DeviceWriteBPs,
		deviceWriteBpsFlagName, []string{},
		"Limit write rate (bytes per second) to a device (e.g. --device-write-bps=/dev/sda:1mb)",
	)
	_ = cmd.RegisterFlagCompletionFunc(deviceWriteBpsFlagName, completion.AutocompleteDefault)

	blkioWeightFlagName := "blkio-weight"
	createFlags.StringVar(
		&cf.BlkIOWeight,
		blkioWeightFlagName, "",
		"Block IO weight (relative weight) accepts a weight value between 10 and 1000.",
	)
	_ = cmd.RegisterFlagCompletionFunc(blkioWeightFlagName, completion.AutocompleteNone)

	blkioWeightDeviceFlagName := "blkio-weight-device"
	createFlags.StringArrayVar(
		&cf.BlkIOWeightDevice,
		blkioWeightDeviceFlagName, []string{},
		"Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)",
	)
	_ = cmd.RegisterFlagCompletionFunc(blkioWeightDeviceFlagName, completion.AutocompleteDefault)
}

func GetHealthCheckOverrideConfig(cmd *cobra.Command, vals *entities.ContainerCreateOptions) (*manifest.Schema2HealthConfig, error) {
	healthcheck := manifest.Schema2HealthConfig{}

	if cmd.Flags().Changed("health-interval") {
		if vals.HealthInterval == "disable" {
			vals.HealthInterval = "-1s"
		}
		hct, err := time.ParseDuration(vals.HealthInterval)
		if err != nil {
			return nil, err
		}
		healthcheck.Interval = hct
	}
	if cmd.Flags().Changed("health-retries") {
		healthcheck.Retries = int(vals.HealthRetries)
	}
	if cmd.Flags().Changed("health-timeout") {
		hct, err := time.ParseDuration(vals.HealthTimeout)
		if err != nil {
			return nil, err
		}
		if hct < time.Duration(1) {
			return nil, errors.New("healthcheck-timeout must be at least 1 second")
		}
		healthcheck.Timeout = hct
	}
	if cmd.Flags().Changed("health-start-period") {
		hct, err := time.ParseDuration(vals.HealthStartPeriod)
		if err != nil {
			return nil, err
		}
		if hct < time.Duration(0) {
			return nil, errors.New("healthcheck-start-period must be 0 seconds or greater")
		}
		healthcheck.StartPeriod = hct
	}

	return &healthcheck, nil
}
