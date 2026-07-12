package quadlet

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	quadletListDescription = `List all Quadlets configured for the current user.`

	quadletListCmd = &cobra.Command{
		Use:               "list [options]",
		Aliases:           []string{"ls"},
		Short:             "List Quadlets",
		Long:              quadletListDescription,
		RunE:              list,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman quadlet list
podman quadlet list --format '{{ .UnitName }}'
podman quadlet list --filter 'name=test*'`,
	}

	listOptions entities.QuadletListOptions
	format      string
)

func listFlags(cmd *cobra.Command) {
	formatFlagName := "format"
	filterFlagName := "filter"
	flags := cmd.Flags()

	flags.StringArrayVarP(&listOptions.Filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	flags.StringVar(&format, formatFlagName, "{{range .}}{{.Name}}\t{{.UnitName}}\t{{.Path}}\t{{.Status}}\t{{.App}}\t{{.Pod}}\n{{end -}}", "Pretty-print output to JSON or using a Go template")
	_ = quadletListCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ListQuadlet{}))
	_ = quadletListCmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteQuadletFilters)

	flags.BoolP("noheading", "n", false, "Do not print headers")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletListCmd,
		Parent:  quadletCmd,
	})
	listFlags(quadletListCmd)
}

func list(cmd *cobra.Command, _ []string) error {
	quadlets, err := registry.ContainerEngine().QuadletList(registry.Context(), listOptions)
	if err != nil {
		return err
	}

	if report.IsJSON(format) {
		return outputJSON(quadlets)
	}
	return outputTemplate(cmd, quadlets)
}

func outputTemplate(cmd *cobra.Command, responses []*entities.ListQuadlet) error {
	renderHeaders := true
	if noHeading, _ := cmd.Flags().GetBool("noheading"); noHeading {
		renderHeaders = false
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	origin := report.OriginPodman
	if cmd.Flag("format").Changed {
		origin = report.OriginUser
	}
	rpt, err = rpt.Parse(origin, format)
	if err != nil {
		return err
	}

	if renderHeaders && rpt.RenderHeaders {
		headers := report.Headers(entities.ListQuadlet{}, map[string]string{
			"Name":     "NAME",
			"UnitName": "UNIT NAME",
			"Path":     "PATH ON DISK",
			"Status":   "STATUS",
			"App":      "APPLICATION",
			"Pod":      "POD",
		})

		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("writing column headers: %w", err)
		}
	}

	return rpt.Execute(responses)
}

func outputJSON(vols []*entities.ListQuadlet) error {
	b, err := json.MarshalIndent(vols, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
