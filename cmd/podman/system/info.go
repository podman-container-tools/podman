package system

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"sigs.k8s.io/yaml"
)

var (
	infoDescription = `Display information pertaining to the host, current storage stats, and build of podman.

  Useful for the user and when reporting issues.
`
	infoCommand = &cobra.Command{
		Use:               "info [options]",
		Args:              validate.NoArgs,
		Long:              infoDescription,
		Short:             "Display podman system information",
		RunE:              info,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman info`,
	}

	systemInfoCommand = &cobra.Command{
		Args:              infoCommand.Args,
		Use:               infoCommand.Use,
		Short:             infoCommand.Short,
		Long:              infoCommand.Long,
		RunE:              infoCommand.RunE,
		ValidArgsFunction: infoCommand.ValidArgsFunction,
		Example:           `podman system info`,
	}
)

var (
	inFormat string
	debug    bool
)

type infoReport struct {
	define.Info
	Client *define.Version `json:",omitempty" yaml:",omitempty"`
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: infoCommand,
	})
	infoFlags(infoCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemInfoCommand,
		Parent:  systemCmd,
	})
	infoFlags(systemInfoCommand)
}

func infoFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&debug, "debug", "D", false, "Display additional debug information")
	_ = flags.MarkHidden("debug") // It's a NOP since Podman version 2.0

	formatFlagName := "format"
	flags.StringVarP(&inFormat, formatFlagName, "f", "", "Change the output format to JSON or a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&define.Info{}))
}

func info(cmd *cobra.Command, _ []string) error {
	info, err := registry.ContainerEngine().Info(registry.Context())
	if err != nil {
		return err
	}
	remote := registry.IsRemote()
	info.Host.ServiceIsRemote = remote

	infoReport := infoReport{
		Info: *info,
	}

	if remote {
		clientVers, _ := define.GetVersion()
		infoReport.Client = &clientVers
	}

	switch {
	case report.IsJSON(inFormat):
		b, err := json.MarshalIndent(infoReport, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case cmd.Flags().Changed("format"):
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		// Use OriginUnknown so it does not add an extra range since it
		// will only be called for a single element and not a slice.
		rpt, err = rpt.Parse(report.OriginUnknown, inFormat)
		if err != nil {
			return err
		}
		return rpt.Execute(infoReport)
	default:
		b, err := yaml.Marshal(infoReport)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}
