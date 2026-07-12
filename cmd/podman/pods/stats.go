package pods

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

type podStatsOptionsWrapper struct {
	entities.PodStatsOptions

	// Format - pretty-print to JSON or a go template.
	Format string
	// NoReset - do not reset the screen when streaming.
	NoReset bool
	// NoStream - do not stream stats but write them once.
	NoStream bool
}

var (
	statsOptions     = podStatsOptionsWrapper{}
	statsDescription = `Display the containers' resource-usage statistics of one or more running pod`
	// Command: podman pod _pod_
	statsCmd = &cobra.Command{
		Use:               "stats [options] [POD...]",
		Short:             "Display a live stream of resource usage statistics for the containers in one or more pods",
		Long:              statsDescription,
		RunE:              stats,
		ValidArgsFunction: common.AutocompletePodsRunning,
		Example: `podman pod stats
podman pod stats a69b23034235 named-pod
podman pod stats --all`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: statsCmd,
		Parent:  podCmd,
	})

	flags := statsCmd.Flags()
	flags.BoolVarP(&statsOptions.All, "all", "a", false, "Provide stats for all pods")

	formatFlagName := "format"
	flags.StringVar(&statsOptions.Format, formatFlagName, "", "Pretty-print container statistics to JSON or using a Go template")
	_ = statsCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.PodStatsReport{}))

	flags.BoolVar(&statsOptions.NoReset, "no-reset", false, "Disable resetting the screen when streaming")
	flags.BoolVar(&statsOptions.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result")
	validate.AddLatestFlag(statsCmd, &statsOptions.Latest)
}

func stats(cmd *cobra.Command, args []string) error {
	// Validate input.
	if err := entities.ValidatePodStatsOptions(args, &statsOptions.PodStatsOptions); err != nil {
		return err
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	doJSON := report.IsJSON(statsOptions.Format)
	if !doJSON {
		if cmd.Flags().Changed("format") {
			rpt, err = rpt.Parse(report.OriginUser, statsOptions.Format)
			if err != nil {
				return err
			}
		} else {
			rpt = rpt.Init(os.Stdout, 12, 2, 2, ' ', 0)
			rpt.Origin = report.OriginPodman
		}
	}

	for ; ; time.Sleep(time.Second) {
		reports, err := registry.ContainerEngine().PodStats(context.Background(), args, statsOptions.PodStatsOptions)
		if err != nil {
			return err
		}
		// Print the stats in the requested format and configuration.
		if doJSON {
			err = printJSONPodStats(reports)
		} else {
			if !statsOptions.NoReset {
				common.ClearScreen()
			}
			if report.OriginUser == rpt.Origin {
				err = userTemplate(rpt, reports)
			} else {
				err = defaultTemplate(rpt, reports)
			}
		}
		if err != nil {
			return err
		}
		if statsOptions.NoStream {
			break
		}
	}
	return nil
}

func printJSONPodStats(stats []*entities.PodStatsReport) error {
	b, err := json.MarshalIndent(&stats, "", "     ")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s\n", string(b))
	return nil
}

func defaultTemplate(rpt *report.Formatter, stats []*entities.PodStatsReport) error {
	outFormat := "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(rpt.Writer(), outFormat, "POD", "CID", "NAME", "CPU %", "MEM USAGE/ LIMIT", "MEM %", "NET IO", "BLOCK IO", "PIDS")
	if len(stats) == 0 {
		fmt.Fprintf(rpt.Writer(), outFormat, "--", "--", "--", "--", "--", "--", "--", "--", "--")
	} else {
		for _, i := range stats {
			fmt.Fprintf(rpt.Writer(), outFormat, i.Pod, i.CID, i.Name, i.CPU, i.MemUsage, i.Mem, i.NetIO, i.BlockIO, i.PIDS)
		}
	}
	return rpt.Flush()
}

func userTemplate(rpt *report.Formatter, stats []*entities.PodStatsReport) error {
	if len(stats) == 0 {
		return nil
	}

	headers := report.Headers(entities.PodStatsReport{}, map[string]string{
		"CPU":           "CPU %",
		"MemUsage":      "MEM USAGE/ LIMIT",
		"MemUsageBytes": "MEM USAGE/ LIMIT",
		"MEM":           "MEM %",
		"NET IO":        "NET IO",
		"BlockIO":       "BLOCK IO",
	})

	if err := rpt.Execute(headers); err != nil {
		return err
	}
	if err := rpt.Execute(stats); err != nil {
		return err
	}
	return rpt.Flush()
}
