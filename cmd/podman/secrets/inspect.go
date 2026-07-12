package secrets

import (
	"context"
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
	Use:               "inspect [options] SECRET [SECRET...]",
	Short:             "Inspect a secret",
	Long:              "Display detail information on one or more secrets",
	RunE:              inspect,
	Example:           "podman secret inspect MYSECRET",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: common.AutocompleteSecrets,
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

var inspectOpts = entities.SecretInspectOptions{}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  secretCmd,
	})
	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVarP(&format, formatFlagName, "f", "", "Format inspect output using Go template")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.SecretInfoReport{}))

	flags.BoolVar(&inspectOpts.ShowSecret, "showsecret", false, "Display the secret")

	prettyFlagName := "pretty"
	flags.BoolVar(&pretty, prettyFlagName, false, "Print inspect output in human-readable format")
}

func inspect(cmd *cobra.Command, args []string) error {
	inspected, errs, _ := registry.ContainerEngine().SecretInspect(context.Background(), args, inspectOpts)

	// always print valid list
	if len(inspected) == 0 {
		inspected = []*entities.SecretInfoReport{}
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
				fmt.Fprintf(os.Stderr, "error inspecting secret: %v\n", err)
			}
		}
		return fmt.Errorf("inspecting secret: %w", errs[0])
	}
	return nil
}
