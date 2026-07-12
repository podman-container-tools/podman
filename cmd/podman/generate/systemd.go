package generate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	envLib "go.podman.io/podman/v6/pkg/env"
	systemDefine "go.podman.io/podman/v6/pkg/systemd/define"
)

const (
	startTimeoutFlagName      = "start-timeout"
	stopTimeoutFlagName       = "stop-timeout"
	stopTimeoutCompatFlagName = "time"
	restartPolicyFlagName     = "restart-policy"
	restartSecFlagName        = "restart-sec"
	newFlagName               = "new"
	wantsFlagName             = "wants"
	afterFlagName             = "after"
	requiresFlagName          = "requires"
	envFlagName               = "env"
)

var (
	deprecation = `
DEPRECATED command:
It is recommended to use Quadlets for running containers and pods under systemd.

Please refer to podman-systemd.unit(5) for details.
`

	envInput           []string
	files              bool
	format             string
	systemdRestart     string
	systemdRestartSec  uint
	startTimeout       uint
	stopTimeout        uint
	systemdOptions     = entities.GenerateSystemdOptions{}
	systemdDescription = `Generate systemd units for a pod or container.
  The generated units can later be controlled via systemctl(1).
` + deprecation

	systemdCmd = &cobra.Command{
		Use:               "systemd [options] {CONTAINER|POD}",
		Short:             "[DEPRECATED] Generate systemd units",
		Long:              systemdDescription,
		RunE:              systemd,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteContainersAndPods,
		Example: `podman generate systemd CTR
podman generate systemd --new --time 10 CTR
podman generate systemd --files --name POD`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemdCmd,
		Parent:  GenerateCmd,
	})
	flags := systemdCmd.Flags()
	flags.BoolVarP(&systemdOptions.Name, "name", "n", false, "Use container/pod names instead of IDs")
	flags.BoolVarP(&files, "files", "f", false, "Generate .service files instead of printing to stdout")
	flags.BoolVar(&systemdOptions.TemplateUnitFile, "template", false, "Make it a template file and use %i and %I specifiers. Working only for containers")

	flags.UintVarP(&startTimeout, startTimeoutFlagName, "", 0, "Start timeout override")
	_ = systemdCmd.RegisterFlagCompletionFunc(startTimeoutFlagName, completion.AutocompleteNone)

	// NOTE: initially, there was only a --time/-t flag which mapped to
	// stop-timeout. To remain backwards compatible create a hidden flag
	// that maps to StopTimeout.
	flags.UintVarP(&stopTimeout, stopTimeoutFlagName, "", containerConfig.Engine.StopTimeout, "Stop timeout override")
	_ = systemdCmd.RegisterFlagCompletionFunc(stopTimeoutFlagName, completion.AutocompleteNone)
	flags.UintVarP(&stopTimeout, stopTimeoutCompatFlagName, "t", containerConfig.Engine.StopTimeout, "Backwards alias for --stop-timeout")
	_ = flags.MarkHidden("time")

	flags.BoolVar(&systemdOptions.New, newFlagName, false, "Create a new container or pod instead of starting an existing one")
	flags.BoolVarP(&systemdOptions.NoHeader, "no-header", "", false, "Skip header generation")

	containerPrefixFlagName := "container-prefix"
	flags.StringVar(&systemdOptions.ContainerPrefix, containerPrefixFlagName, "container", "Systemd unit name prefix for containers")
	_ = systemdCmd.RegisterFlagCompletionFunc(containerPrefixFlagName, completion.AutocompleteNone)

	podPrefixFlagName := "pod-prefix"
	flags.StringVar(&systemdOptions.PodPrefix, podPrefixFlagName, "pod", "Systemd unit name prefix for pods")
	_ = systemdCmd.RegisterFlagCompletionFunc(podPrefixFlagName, completion.AutocompleteNone)

	separatorFlagName := "separator"
	flags.StringVar(&systemdOptions.Separator, separatorFlagName, "-", "Systemd unit name separator between name/id and prefix")
	_ = systemdCmd.RegisterFlagCompletionFunc(separatorFlagName, completion.AutocompleteNone)

	flags.StringVar(&systemdRestart, restartPolicyFlagName, systemDefine.DefaultRestartPolicy, "Systemd restart-policy")
	_ = systemdCmd.RegisterFlagCompletionFunc(restartPolicyFlagName, common.AutocompleteSystemdRestartOptions)

	flags.UintVarP(&systemdRestartSec, restartSecFlagName, "", 0, "Systemd restart-sec")
	_ = systemdCmd.RegisterFlagCompletionFunc(restartSecFlagName, completion.AutocompleteNone)

	formatFlagName := "format"
	flags.StringVar(&format, formatFlagName, "", "Print the created units in specified format (json)")
	_ = systemdCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	flags.StringArrayVar(&systemdOptions.Wants, wantsFlagName, nil, "Add (weak) requirement dependencies to the generated unit file")
	_ = systemdCmd.RegisterFlagCompletionFunc(wantsFlagName, completion.AutocompleteNone)

	flags.StringArrayVar(&systemdOptions.After, afterFlagName, nil, "Add dependencies order to the generated unit file")
	_ = systemdCmd.RegisterFlagCompletionFunc(afterFlagName, completion.AutocompleteNone)

	flags.StringArrayVar(&systemdOptions.Requires, requiresFlagName, nil, "Similar to wants, but declares stronger requirement dependencies")
	_ = systemdCmd.RegisterFlagCompletionFunc(requiresFlagName, completion.AutocompleteNone)

	flags.StringArrayVarP(&envInput, envFlagName, "e", nil, "Set environment variables to the systemd unit files")
	_ = systemdCmd.RegisterFlagCompletionFunc(envFlagName, completion.AutocompleteNone)

	flags.SetNormalizeFunc(utils.TimeoutAliasFlags)
}

func systemd(cmd *cobra.Command, args []string) error {
	fmt.Fprint(os.Stderr, deprecation)
	if cmd.Flags().Changed(restartPolicyFlagName) {
		systemdOptions.RestartPolicy = &systemdRestart
	}

	if registry.IsRemote() {
		logrus.Warnln("The generated units should be placed on your remote system")
	}

	if cmd.Flags().Changed(newFlagName) && !systemdOptions.New && systemdOptions.TemplateUnitFile {
		return errors.New("--template cannot be set with --new=false")
	}
	if !systemdOptions.New && systemdOptions.TemplateUnitFile {
		systemdOptions.New = true
	}

	if cmd.Flags().Changed(restartSecFlagName) {
		systemdOptions.RestartSec = &systemdRestartSec
	}
	if cmd.Flags().Changed(startTimeoutFlagName) {
		systemdOptions.StartTimeout = &startTimeout
	}
	setStopTimeout := 0
	if cmd.Flags().Changed(stopTimeoutFlagName) {
		setStopTimeout++
	}
	if cmd.Flags().Changed(stopTimeoutCompatFlagName) {
		setStopTimeout++
	}
	if cmd.Flags().Changed(envFlagName) {
		cliEnv, err := envLib.ParseSlice(envInput)
		if err != nil {
			return fmt.Errorf("parsing environment variables: %w", err)
		}
		systemdOptions.AdditionalEnvVariables = envLib.Slice(cliEnv)
	}
	switch setStopTimeout {
	case 1:
		systemdOptions.StopTimeout = &stopTimeout
	case 2:
		return fmt.Errorf("%s and %s are redundant and cannot be used together", stopTimeoutFlagName, stopTimeoutCompatFlagName)
	}

	reports, err := registry.ContainerEngine().GenerateSystemd(registry.Context(), args[0], systemdOptions)
	if err != nil {
		return err
	}

	if files {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current working directory: %w", err)
		}
		for name, content := range reports.Units {
			path := filepath.Join(cwd, fmt.Sprintf("%s.service", name))
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			_, err = f.WriteString(content)
			if err != nil {
				return err
			}
			err = f.Close()
			if err != nil {
				return err
			}

			// add newline if default format is given
			if format == "" {
				path += "\n"
			}
			// modify in place so we can print the
			// paths when --files is set
			reports.Units[name] = path
		}
	}

	switch {
	case report.IsJSON(format):
		return printJSON(reports.Units)
	case format == "":
		return printDefault(reports.Units)
	default:
		return fmt.Errorf("unknown --format argument: %s", format)
	}
}

func printDefault(units map[string]string) error {
	for _, content := range units {
		fmt.Print(content)
	}
	return nil
}

func printJSON(units map[string]string) error {
	b, err := json.MarshalIndent(units, "", " ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
