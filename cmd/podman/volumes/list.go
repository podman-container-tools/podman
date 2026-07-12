package volumes

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	volumeLsDescription = `
podman volume ls

List all available volumes. The output of the volumes can be filtered
and the output format can be changed to JSON or a user specified Go template.`
	lsCommand = &cobra.Command{
		Use:               "ls [options]",
		Aliases:           []string{"list"},
		Args:              validate.NoArgs,
		Short:             "List volumes",
		Long:              volumeLsDescription,
		RunE:              list,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	// Temporary struct to hold cli values.
	cliOpts = struct {
		Filter []string
		Format string
		Quiet  bool
	}{}
	lsOpts = entities.VolumeListOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCommand,
		Parent:  volumeCmd,
	})
	flags := lsCommand.Flags()

	filterFlagName := "filter"
	flags.StringArrayVarP(&cliOpts.Filter, filterFlagName, "f", []string{}, "Filter volume output")
	_ = lsCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteVolumeFilters)

	formatFlagName := "format"
	flags.StringVar(&cliOpts.Format, formatFlagName, "{{range .}}{{.Driver}}\t{{.Name}}\n{{end -}}", "Format volume output using Go template")
	_ = lsCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.VolumeListReport{}))

	flags.BoolP("noheading", "n", false, "Do not print headers")
	flags.BoolVarP(&cliOpts.Quiet, "quiet", "q", false, "Print volume output in quiet mode")
}

func list(cmd *cobra.Command, _ []string) error {
	var err error
	if cliOpts.Quiet && cmd.Flag("format").Changed {
		return errors.New("quiet and format flags cannot be used together")
	}
	if len(cliOpts.Filter) > 0 {
		lsOpts.Filter = make(map[string][]string)
	}
	lsOpts.Filter, err = parse.FilterArgumentsIntoFilters(cliOpts.Filter)
	if err != nil {
		return err
	}

	responses, err := registry.ContainerEngine().VolumeList(context.Background(), lsOpts)
	if err != nil {
		return err
	}

	if report.IsJSON(cliOpts.Format) {
		return outputJSON(responses)
	}
	return outputTemplate(cmd, responses)
}

func outputTemplate(cmd *cobra.Command, responses []*entities.VolumeListReport) error {
	noHeading, _ := cmd.Flags().GetBool("noheading")
	headers := report.Headers(entities.VolumeListReport{}, map[string]string{
		"Name": "VOLUME NAME",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, cliOpts.Format)
	case cliOpts.Quiet:
		rpt, err = rpt.Parse(report.OriginUser, "{{.Name}}\n")
	default:
		rpt, err = rpt.Parse(report.OriginPodman, cliOpts.Format)
	}
	if err != nil {
		return err
	}

	if (rpt.RenderHeaders) && !noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	reporters := make([]volumeReporter, 0, len(responses))
	for _, r := range responses {
		reporters = append(reporters, volumeReporter{r})
	}
	return rpt.Execute(reporters)
}

type volumeReporter struct {
	*entities.VolumeListReport
}

// Labels returns the volume's labels as a sorted, comma-separated list of
// key=value pairs, matching Docker CLI output format.
func (v volumeReporter) Labels() string {
	return common.FormatLabels(v.VolumeListReport.Labels)
}

func outputJSON(vols []*entities.VolumeListReport) error {
	b, err := json.MarshalIndent(vols, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
