//go:build !remote && (linux || freebsd)

package generate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/libpod"
	libpodDefine "go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/systemd/define"
	"go.podman.io/podman/v6/version"
)

// containerInfo contains data required for generating a container's systemd
// unit file.
type containerInfo struct {
	ServiceName            string
	ContainerNameOrID      string
	Type                   string
	NotifyAccess           string
	StopTimeout            uint
	RestartPolicy          string
	RestartSec             uint
	StartLimitBurst        string
	PIDFile                string
	ContainerIDFile        string
	GenerateTimestamp      bool
	BoundToServices        []string
	PodmanVersion          string
	Executable             string
	RootFlags              string
	TimeStamp              string
	CreateCommand          []string
	containerEnv           []string
	ExtraEnvs              []string
	EnvVariable            string
	AdditionalEnvVariables []string
	ExecStart              string
	TimeoutStartSec        uint
	TimeoutStopSec         uint
	ExecStartPre           string
	ExecStop               string
	ExecStopPost           string
	GenerateNoHeader       bool
	Pod                    *podInfo
	GraphRoot              string
	RunRoot                string
	IdentifySpecifier      bool
	Wants                  []string
	After                  []string
	Requires               []string
}

const containerTemplate = headerTemplate + `
{{{{- if .BoundToServices}}}}
BindsTo={{{{- range $index, $value := .BoundToServices -}}}}{{{{if $index}}}} {{{{end}}}}{{{{ $value }}}}.service{{{{end}}}}
After={{{{- range $index, $value := .BoundToServices -}}}}{{{{if $index}}}} {{{{end}}}}{{{{ $value }}}}.service{{{{end}}}}
{{{{- end}}}}
{{{{- if or .Wants .After .Requires }}}}

# User-defined dependencies
{{{{- end}}}}
{{{{- if .Wants}}}}
Wants={{{{- range $index, $value := .Wants }}}}{{{{ if $index}}}} {{{{end}}}}{{{{ $value }}}}{{{{end}}}}
{{{{- end}}}}
{{{{- if .After}}}}
After={{{{- range $index, $value := .After }}}}{{{{ if $index}}}} {{{{end}}}}{{{{ $value }}}}{{{{end}}}}
{{{{- end}}}}
{{{{- if .Requires}}}}
Requires={{{{- range $index, $value := .Requires }}}}{{{{ if $index}}}} {{{{end}}}}{{{{ $value }}}}{{{{end}}}}
{{{{- end}}}}

[Service]
Environment={{{{.EnvVariable}}}}=%n{{{{- if (eq .IdentifySpecifier true) }}}}-%i {{{{- end}}}}
{{{{- if .ExtraEnvs}}}}
Environment={{{{- range $index, $value := .ExtraEnvs -}}}}{{{{if $index}}}} {{{{end}}}}{{{{ $value }}}}{{{{end}}}}
{{{{- end}}}}
{{{{- if .AdditionalEnvVariables}}}}
{{{{- range $index, $value := .AdditionalEnvVariables -}}}}{{{{if $index}}}}{{{{end}}}}
Environment={{{{ $value }}}}{{{{end}}}}
{{{{- end}}}}
Restart={{{{.RestartPolicy}}}}
{{{{- if .RestartSec}}}}
RestartSec={{{{.RestartSec}}}}
{{{{- end}}}}
{{{{- if .StartLimitBurst}}}}
StartLimitBurst={{{{.StartLimitBurst}}}}
{{{{- end}}}}
{{{{- if ne .TimeoutStartSec 0}}}}
TimeoutStartSec={{{{.TimeoutStartSec}}}}
{{{{- end}}}}
TimeoutStopSec={{{{.TimeoutStopSec}}}}
ExecStart={{{{.ExecStart}}}}
{{{{- if .ExecStop}}}}
ExecStop={{{{.ExecStop}}}}
{{{{- end}}}}
{{{{- if .ExecStopPost}}}}
ExecStopPost={{{{.ExecStopPost}}}}
{{{{- end}}}}
{{{{- if .PIDFile}}}}
PIDFile={{{{.PIDFile}}}}
{{{{- end}}}}
Type={{{{.Type}}}}
{{{{- if .NotifyAccess}}}}
NotifyAccess={{{{.NotifyAccess}}}}
{{{{- end}}}}

[Install]
WantedBy=multi-user.target
`

// ContainerUnit generates a systemd unit for the specified container.  Based
// on the options, the return value might be the entire unit or a file it has
// been written to.
func ContainerUnit(ctr *libpod.Container, options entities.GenerateSystemdOptions) (string, string, error) {
	info, err := generateContainerInfo(ctr, options)
	if err != nil {
		return "", "", err
	}
	content, err := executeContainerTemplate(info, options)
	if err != nil {
		return "", "", err
	}
	return info.ServiceName, content, nil
}

func generateContainerInfo(ctr *libpod.Container, options entities.GenerateSystemdOptions) (*containerInfo, error) {
	stopTimeout := ctr.StopTimeout()
	if options.StopTimeout != nil {
		stopTimeout = *options.StopTimeout
	}

	startTimeout := uint(0)
	if options.StartTimeout != nil {
		startTimeout = *options.StartTimeout
	}

	config := ctr.Config()
	if len(config.InitContainerType) > 0 {
		return nil, fmt.Errorf("unsupported container %s: cannot generate systemd units for init containers", ctr.ID())
	}

	conmonPidFile := config.ConmonPidFile
	if conmonPidFile == "" {
		return nil, errors.New("conmon PID file path is empty, try to recreate the container with --conmon-pidfile flag")
	}

	// #15284: old units generated without --new can lead to issues on
	// shutdown when the containers are created with a custom restart
	// policy.
	if !options.New {
		switch config.RestartPolicy {
		case libpodDefine.RestartPolicyNo, libpodDefine.RestartPolicyNone:
			// All good
		default:
			logrus.Warnf("Container %s has restart policy %q which can lead to issues on shutdown: consider recreating the container without a restart policy and use systemd's restart mechanism instead", ctr.ID(), config.RestartPolicy)
		}
	}

	createCommand := []string{}
	if config.CreateCommand != nil {
		createCommand = config.CreateCommand
	} else if options.New {
		return nil, fmt.Errorf("cannot use --new on container %q: no create command found: only works on containers created directly with podman but not via REST API", ctr.ID())
	}

	nameOrID, serviceName := containerServiceName(ctr, options)

	var runRoot string
	if options.New {
		runRoot = "%t/containers"
	} else {
		runRoot = ctr.Runtime().RunRoot()
		if runRoot == "" {
			return nil, errors.New("could not look up container's runroot: got empty string")
		}
	}

	envs := config.Spec.Process.Env

	info := containerInfo{
		ServiceName:            serviceName,
		ContainerNameOrID:      nameOrID,
		RestartPolicy:          define.DefaultRestartPolicy,
		PIDFile:                conmonPidFile,
		TimeoutStartSec:        startTimeout,
		StopTimeout:            stopTimeout,
		GenerateTimestamp:      true,
		CreateCommand:          createCommand,
		RunRoot:                runRoot,
		containerEnv:           envs,
		Wants:                  options.Wants,
		After:                  options.After,
		Requires:               options.Requires,
		AdditionalEnvVariables: options.AdditionalEnvVariables,
	}

	return &info, nil
}

// containerServiceName returns the nameOrID and the service name of the
// container.
func containerServiceName(ctr *libpod.Container, options entities.GenerateSystemdOptions) (string, string) {
	nameOrID := ctr.ID()
	if options.Name {
		nameOrID = ctr.Name()
	}

	serviceName := getServiceName(options.ContainerPrefix, options.Separator, nameOrID)

	return nameOrID, serviceName
}

// setContainerNameForTemplate updates startCommand to contain the name argument with
// a value that includes the identify specifier.
// In case startCommand doesn't contain that argument it's added after "run" and its
// value will be set to info.ServiceName concated with the identify specifier %i.
func setContainerNameForTemplate(startCommand []string, info *containerInfo) ([]string, error) {
	// find the index of "--name" in the command slice
	nameIx := -1
	for argIx, arg := range startCommand {
		if arg == "--name" {
			nameIx = argIx + 1
			break
		}
		if strings.HasPrefix(arg, "--name=") {
			nameIx = argIx
			break
		}
	}
	switch nameIx {
	case -1:
		// if not found, add --name argument in the command slice before the "run" argument.
		// it's assumed that the command slice contains this argument.
		runIx := -1
		for argIx, arg := range startCommand {
			if arg == "run" {
				runIx = argIx
				break
			}
		}
		if runIx == -1 {
			return startCommand, fmt.Errorf("\"run\" is missing in the command arguments")
		}
		startCommand = append(startCommand[:runIx+1], startCommand[runIx:]...)
		startCommand[runIx+1] = fmt.Sprintf("--name=%s-%%i", info.ServiceName)
	default:
		// append the identity specifier (%i) to the end of the --name value
		startCommand[nameIx] = fmt.Sprintf("%s-%%i", startCommand[nameIx])
	}
	return startCommand, nil
}

func formatOptionsString(cmd string) string {
	return formatOptions(strings.Split(cmd, " "))
}

func formatOptions(options []string) string {
	var formatted strings.Builder
	if len(options) == 0 {
		return ""
	}
	formatted.WriteString(options[0])
	for _, o := range options[1:] {
		if strings.HasPrefix(o, "-") {
			formatted.WriteString(" \\\n\t" + o)
			continue
		}
		formatted.WriteString(" " + o)
	}
	return formatted.String()
}

// executeContainerTemplate executes the container template on the specified
// containerInfo.  Note that the containerInfo is also post processed and
// completed, which allows for an easier unit testing.
func executeContainerTemplate(info *containerInfo, options entities.GenerateSystemdOptions) (string, error) {
	if options.RestartPolicy != nil {
		if err := validateRestartPolicy(*options.RestartPolicy); err != nil {
			return "", err
		}
		info.RestartPolicy = *options.RestartPolicy
	}

	if options.RestartSec != nil {
		info.RestartSec = *options.RestartSec
	}

	// Make sure the executable is set.
	if info.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			executable = "/usr/bin/podman"
			logrus.Warnf("Could not obtain podman executable location, using default %s", executable)
		}
		info.Executable = executable
	}

	info.Type = "forking"
	info.EnvVariable = define.EnvVariable
	info.ExecStart = "{{{{.Executable}}}} start {{{{.ContainerNameOrID}}}}"
	info.ExecStop = formatOptionsString("{{{{.Executable}}}} stop {{{{if (ge .StopTimeout 0)}}}} -t {{{{.StopTimeout}}}}{{{{end}}}} {{{{.ContainerNameOrID}}}}")
	info.ExecStopPost = formatOptionsString("{{{{.Executable}}}} stop {{{{if (ge .StopTimeout 0)}}}} -t {{{{.StopTimeout}}}}{{{{end}}}} {{{{.ContainerNameOrID}}}}")
	for i, env := range info.AdditionalEnvVariables {
		info.AdditionalEnvVariables[i] = escapeSystemdArg(env)
	}

	// Assemble the ExecStart command when creating a new container.
	//
	// Note that we cannot catch all corner cases here such that users
	// *must* manually check the generated files.  A container might have
	// been created via a Python script, which would certainly yield an
	// invalid `info.CreateCommand`.  Hence, we're doing a best effort unit
	// generation and don't try aiming at completeness.
	if options.New {
		info.Type = "notify"
		info.NotifyAccess = "all"
		info.PIDFile = ""
		info.ContainerIDFile = "%t/%n.ctr-id"
		info.ExecStop = formatOptionsString("{{{{.Executable}}}} stop --ignore {{{{if (ge .StopTimeout 0)}}}}-t {{{{.StopTimeout}}}}{{{{end}}}} --cidfile={{{{.ContainerIDFile}}}}")
		info.ExecStopPost = formatOptionsString("{{{{.Executable}}}} rm -f --ignore {{{{if (ge .StopTimeout 0)}}}}-t {{{{.StopTimeout}}}}{{{{end}}}} --cidfile={{{{.ContainerIDFile}}}}")
		// The create command must at least have three arguments:
		// 	/usr/bin/podman run $IMAGE
		index := 0
		for i, arg := range info.CreateCommand {
			if arg == "run" || arg == "create" {
				index = i + 1
				break
			}
		}
		if index == 0 {
			return "", fmt.Errorf("container's create command is too short or invalid: %v", info.CreateCommand)
		}
		// We're hard-coding the first five arguments and append the
		// CreateCommand with a stripped command and subcommand.
		startCommand := []string{info.Executable}
		if index > 2 {
			// include root flags
			info.RootFlags = strings.Join(escapeSystemdArguments(info.CreateCommand[1:index-1]), " ")
			startCommand = append(startCommand, info.CreateCommand[1:index-1]...)
		}
		startCommand = append(startCommand,
			"run",
			"--cidfile={{{{.ContainerIDFile}}}}",
			"--cgroups=no-conmon",
			"--rm",
		)
		remainingCmd := info.CreateCommand[index:]
		// Presence check for certain flags/options.
		fs := pflag.NewFlagSet("args", pflag.ContinueOnError)
		fs.ParseErrorsAllowlist.UnknownFlags = true
		fs.Usage = func() {}
		fs.SetInterspersed(false)
		fs.BoolP("detach", "d", false, "")
		fs.String("name", "", "")
		fs.Bool("replace", false, "")
		fs.StringArrayP("env", "e", nil, "")
		fs.String("sdnotify", "", "")
		fs.String("restart", "", "")
		// have to define extra -h flag to prevent help error when parsing -h hostname
		// https://github.com/containers/podman/issues/15124
		fs.StringP("help", "h", "", "")
		if err := fs.Parse(remainingCmd); err != nil {
			return "", fmt.Errorf("parsing remaining command-line arguments: %w", err)
		}

		remainingCmd = filterCommonContainerFlags(remainingCmd, fs.NArg())
		// If the container is in a pod, make sure that the
		// --pod-id-file is set correctly.
		if info.Pod != nil {
			podFlags := []string{"--pod-id-file", "{{{{.Pod.PodIDFile}}}}"}
			startCommand = append(startCommand, podFlags...)
			remainingCmd = filterPodFlags(remainingCmd, fs.NArg())
		}

		hasDetachParam, err := fs.GetBool("detach")
		if err != nil {
			return "", err
		}
		hasNameParam := fs.Lookup("name").Changed
		hasReplaceParam, err := fs.GetBool("replace")
		if err != nil {
			return "", err
		}

		// Default to --sdnotify=conmon unless already set by the
		// container.
		sdnotifyFlag := fs.Lookup("sdnotify")
		if !sdnotifyFlag.Changed {
			startCommand = append(startCommand, "--sdnotify=conmon")
		} else if sdnotifyFlag.Value.String() == libpodDefine.SdNotifyModeIgnore {
			// If ignore is set force conmon otherwise the unit with Type=notify will fail.
			logrus.Infof("Forcing --sdnotify=conmon for container %s", info.ContainerNameOrID)
			remainingCmd = removeSdNotifyArg(remainingCmd, fs.NArg())
			startCommand = append(startCommand, "--sdnotify=conmon")
		}

		if !hasDetachParam {
			// Enforce detaching
			//
			// since we use systemd `Type=forking` service @see
			// https://www.freedesktop.org/software/systemd/man/systemd.service.html#Type=
			// when we generated systemd service file with the
			// --new param, `ExecStart` will have `/usr/bin/podman
			// run ...` if `info.CreateCommand` has no `-d` or
			// `--detach` param, podman will run the container in
			// default attached mode, as a result, `systemd start`
			// will wait the `podman run` command exit until failed
			// with timeout error.
			startCommand = append(startCommand, "-d")

			if fs.Changed("detach") {
				// this can only happen if --detach=false is set
				// in that case we need to remove it otherwise we
				// would overwrite the previous detach arg to false
				remainingCmd = removeDetachArg(remainingCmd, fs.NArg())
			}
		}
		if hasNameParam && !hasReplaceParam {
			// Enforce --replace for named containers.  This will
			// make systemd units more robust as it allows them to
			// start after system crashes (see
			// github.com/containers/podman/issues/5485).
			startCommand = append(startCommand, "--replace")

			if fs.Changed("replace") {
				// this can only happen if --replace=false is set
				// in that case we need to remove it otherwise we
				// would overwrite the previous replace arg to false
				remainingCmd = removeReplaceArg(remainingCmd, fs.NArg())
			}
		}

		// Unless the user explicitly set a restart policy, check
		// whether the container was created with a custom one and use
		// it instead of the default.
		if options.RestartPolicy == nil {
			restartPolicy, err := fs.GetString("restart")
			if err != nil {
				return "", err
			}
			if restartPolicy != "" {
				if strings.HasPrefix(restartPolicy, "on-failure:") {
					// Special case --restart=on-failure:5
					spl := strings.Split(restartPolicy, ":")
					restartPolicy = spl[0]
					info.StartLimitBurst = spl[1]
				} else if restartPolicy == libpodDefine.RestartPolicyUnlessStopped {
					restartPolicy = libpodDefine.RestartPolicyAlways
				}
				info.RestartPolicy = restartPolicy
			}
		}

		envs, err := fs.GetStringArray("env")
		if err != nil {
			return "", err
		}
		for _, env := range envs {
			// if env arg does not contain an equal sign we have to add the envar to the unit
			// because it does try to red the value from the environment
			if !strings.Contains(env, "=") {
				for _, containerEnv := range info.containerEnv {
					key, _, _ := strings.Cut(containerEnv, "=")
					if key == env {
						info.ExtraEnvs = append(info.ExtraEnvs, escapeSystemdArg(containerEnv))
					}
				}
			}
		}

		startCommand = append(startCommand, remainingCmd...)
		startCommand = escapeSystemdArguments(startCommand)
		if options.TemplateUnitFile {
			info.IdentifySpecifier = true
			startCommand, err = setContainerNameForTemplate(startCommand, info)
			if err != nil {
				return "", err
			}
		}
		info.ExecStart = formatOptions(startCommand)
	}
	info.TimeoutStopSec = minTimeoutStopSec + info.StopTimeout

	if info.PodmanVersion == "" {
		info.PodmanVersion = version.Version.String()
	}

	if options.NoHeader {
		info.GenerateNoHeader = true
		info.GenerateTimestamp = false
	}

	if info.GenerateTimestamp {
		info.TimeStamp = time.Now().Format(time.UnixDate)
	}
	// Sort the slices to assure a deterministic output.
	sort.Strings(info.BoundToServices)

	// Generate the template and compile it.
	//
	// Note that we need a two-step generation process to allow for fields
	// embedding other fields.  This way we can replace `A -> B -> C` and
	// make the code easier to maintain at the cost of a slightly slower
	// generation.  That's especially needed for embedding the PID and ID
	// files in other fields which will eventually get replaced in the 2nd
	// template execution.
	templ, err := template.New("container_template").Delims("{{{{", "}}}}").Parse(containerTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing systemd service template: %w", err)
	}

	var buf bytes.Buffer
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	// Now parse the generated template (i.e., buf) and execute it.
	templ, err = template.New("container_template").Delims("{{{{", "}}}}").Parse(buf.String())
	if err != nil {
		return "", fmt.Errorf("parsing systemd service template: %w", err)
	}

	buf = bytes.Buffer{}
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	return buf.String(), nil
}
