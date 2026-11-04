package configs

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var inspectCmd = &cobra.Command{
	Use:               "inspect [options] CONFIG [CONFIG...]",
	Short:             "Inspect a config",
	Long:              "Display detail information on one or more configs",
	RunE:              inspect,
	Example:           "podman config inspect MYCONFIG",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: common.AutocompleteConfigs,
}

var (
	format string
	pretty bool
)

const (
	prettyTemplate = `ID:              {{.ID}}
Name:              {{.Spec.Name}}
{{- if .Spec.Labels }}
Labels:
{{- range $k, $v := .Spec.Labels }}
 - {{ $k }}{{if $v }}={{ $v }}{{ end }}
{{- end }}{{ end }}
Driver:            {{.Spec.Driver.Name}}
Created at:        {{.CreatedAt}}
Updated at:        {{.UpdatedAt}}`
)

var inspectOpts = entities.ConfigInspectOptions{}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  configCmd,
	})
	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&format, formatFlagName, "f", "", "Format inspect output using Go template")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ConfigInfoReport{}))

	flags.BoolVar(&inspectOpts.ShowConfig, "showconfig", false, "Display the config")

	prettyFlagName := "pretty"
	flags.BoolVar(&pretty, prettyFlagName, false, "Print inspect output in human-readable format")
}

func inspect(cmd *cobra.Command, args []string) error {
	inspected, errs, err := registry.ContainerEngine().ConfigInspect(cmd.Context(), args, inspectOpts)
	if err != nil {
		return err
	}

	if len(inspected) == 0 {
		inspected = []*entities.ConfigInfoReport{}
	}

	switch {
	case cmd.Flags().Changed("pretty"):
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		rpt, err := rpt.Parse(report.OriginUser, prettyTemplate)
		if err != nil {
			return err
		}

		if err := rpt.Execute(inspected); err != nil {
			return err
		}

	case cmd.Flags().Changed("format"):
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		rpt, err := rpt.Parse(report.OriginUser, format)
		if err != nil {
			return err
		}

		if err := rpt.Execute(inspected); err != nil {
			return err
		}

	default:
		buf, err := json.MarshalIndent(inspected, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(buf))
	}

	if len(errs) > 0 {
		if len(errs) > 1 {
			for _, err := range errs[1:] {
				fmt.Fprintf(os.Stderr, "error inspecting config: %v\n", err)
			}
		}
		return fmt.Errorf("inspecting config: %w", errs[0])
	}
	return nil
}
