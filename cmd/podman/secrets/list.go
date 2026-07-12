package secrets

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/docker/go-units"
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
	lsCmd = &cobra.Command{
		Use:               "ls [options]",
		Aliases:           []string{"list"},
		Short:             "List secrets",
		RunE:              ls,
		Example:           "podman secret ls",
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
	}
	listFlag = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
	filter    []string
	quiet     bool
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCmd,
		Parent:  secretCmd,
	})

	flags := lsCmd.Flags()

	formatFlagName := "format"
	flags.StringVar(&listFlag.format, formatFlagName, "{{range .}}{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.CreatedAt}}\t{{.UpdatedAt}}\n{{end -}}", "Format secret output using Go template")
	_ = lsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.SecretListReport{}))

	filterFlagName := "filter"
	flags.StringArrayVarP(&listFlag.filter, filterFlagName, "f", []string{}, "Filter secret output")
	_ = lsCmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteSecretFilters)

	noHeadingFlagName := "noheading"
	flags.BoolVarP(&listFlag.noHeading, noHeadingFlagName, "n", false, "Do not print headers")

	quietFlagName := "quiet"
	flags.BoolVarP(&listFlag.quiet, quietFlagName, "q", false, "Print secret IDs only")
}

func ls(cmd *cobra.Command, _ []string) error {
	var err error
	lsOpts := entities.SecretListRequest{}

	lsOpts.Filters, err = parse.FilterArgumentsIntoFilters(listFlag.filter)
	if err != nil {
		return err
	}

	responses, err := registry.ContainerEngine().SecretList(context.Background(), lsOpts)
	if err != nil {
		return err
	}

	listed := make([]*entities.SecretListReport, 0, len(responses))
	for _, response := range responses {
		listed = append(listed, &entities.SecretListReport{
			ID:        response.ID,
			Name:      response.Spec.Name,
			CreatedAt: units.HumanDuration(time.Since(response.CreatedAt)) + " ago",
			UpdatedAt: units.HumanDuration(time.Since(response.UpdatedAt)) + " ago",
			Driver:    response.Spec.Driver.Name,
		})
	}

	if listFlag.quiet && !cmd.Flags().Changed("format") {
		return quietOut(listed)
	}

	return outputTemplate(cmd, listed)
}

func quietOut(responses []*entities.SecretListReport) error {
	for _, response := range responses {
		fmt.Println(response.ID)
	}
	return nil
}

func outputTemplate(cmd *cobra.Command, responses []*entities.SecretListReport) error {
	headers := report.Headers(entities.SecretListReport{}, map[string]string{
		"CreatedAt": "CREATED",
		"UpdatedAt": "UPDATED",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, listFlag.format)
	default:
		rpt, err = rpt.Parse(report.OriginPodman, listFlag.format)
	}
	if err != nil {
		return err
	}

	noHeading, _ := cmd.Flags().GetBool("noheading")
	if rpt.RenderHeaders && !noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(responses)
}
