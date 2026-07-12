package containers

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	putils "go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	statsDescription = "Display percentage of CPU, memory, network I/O, block I/O and PIDs for one or more containers."
	statsCommand     = &cobra.Command{
		Use:               "stats [options] [CONTAINER...]",
		Short:             "Display a live stream of container resource usage statistics",
		Long:              statsDescription,
		RunE:              stats,
		Args:              checkStatOptions,
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman stats --all --no-stream
podman stats ctrID
podman stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" ctrID`,
	}

	containerStatsCommand = &cobra.Command{
		Use:               statsCommand.Use,
		Short:             statsCommand.Short,
		Long:              statsCommand.Long,
		RunE:              statsCommand.RunE,
		Args:              checkStatOptions,
		ValidArgsFunction: statsCommand.ValidArgsFunction,
		Example: `podman container stats --all --no-stream
podman container stats ctrID
podman container stats --no-stream --format "table {{.ID}} {{.Name}} {{.MemUsage}}" ctrID`,
	}
)

// statsOptionsCLI is used for storing CLI arguments. Some fields are later
// used in the backend.
type statsOptionsCLI struct {
	All      bool
	Format   string
	Latest   bool
	NoReset  bool
	NoStream bool
	Interval int
}

var (
	statsOptions statsOptionsCLI
	notrunc      bool
)

func statFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&statsOptions.All, "all", "a", false, "Show all containers. Only running containers are shown by default. The default is false")

	formatFlagName := "format"
	flags.StringVar(&statsOptions.Format, formatFlagName, "", "Pretty-print container statistics to JSON or using a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&containerStats{}))

	flags.BoolVar(&notrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVar(&statsOptions.NoReset, "no-reset", false, "Disable resetting the screen between intervals")
	flags.BoolVar(&statsOptions.NoStream, "no-stream", false, "Disable streaming stats and only pull the first result, default setting is false")
	intervalFlagName := "interval"
	flags.IntVarP(&statsOptions.Interval, intervalFlagName, "i", 5, "Time in seconds between stats reports")
	_ = cmd.RegisterFlagCompletionFunc(intervalFlagName, completion.AutocompleteNone)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: statsCommand,
	})
	statFlags(statsCommand)
	validate.AddLatestFlag(statsCommand, &statsOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerStatsCommand,
		Parent:  containerCmd,
	})
	statFlags(containerStatsCommand)
	validate.AddLatestFlag(containerStatsCommand, &statsOptions.Latest)
}

// stats is different in that it will assume running containers if
// no input is given, so we need to validate differently
func checkStatOptions(_ *cobra.Command, args []string) error {
	opts := 0
	if statsOptions.All {
		opts++
	}
	if statsOptions.Latest {
		opts++
	}
	if len(args) > 0 {
		opts++
	}
	if opts > 1 {
		return errors.New("--all, --latest and containers cannot be used together")
	}
	return nil
}

func stats(cmd *cobra.Command, args []string) error {
	// Convert to the entities options.  We should not leak CLI-only
	// options into the backend and separate concerns.
	opts := entities.ContainerStatsOptions{
		Latest:   statsOptions.Latest,
		Stream:   !statsOptions.NoStream,
		Interval: statsOptions.Interval,
		All:      statsOptions.All,
	}
	args = putils.RemoveSlash(args)
	statsChan, err := registry.ContainerEngine().ContainerStats(registry.Context(), args, opts)
	if err != nil {
		return err
	}
	for report := range statsChan {
		if report.Error != nil {
			return report.Error
		}
		if err := outputStats(cmd, report.Stats); err != nil {
			return err
		}
	}
	return nil
}

func outputStats(cmd *cobra.Command, reports []define.ContainerStats) error {
	headers := report.Headers(define.ContainerStats{}, map[string]string{
		"ID":            "ID",
		"UpTime":        "CPU TIME",
		"CPUPerc":       "CPU %",
		"AVGCPU":        "Avg CPU %",
		"MemUsage":      "MEM USAGE / LIMIT",
		"MemUsageBytes": "MEM USAGE / LIMIT",
		"MemPerc":       "MEM %",
		"NetIO":         "NET IO",
		"BlockIO":       "BLOCK IO",
		"PIDS":          "PIDS",
	})
	if !statsOptions.NoReset {
		common.ClearScreen()
	}
	stats := make([]containerStats, 0, len(reports))
	for _, r := range reports {
		stats = append(stats, containerStats{r})
	}
	if report.IsJSON(statsOptions.Format) {
		return outputJSON(stats)
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	if cmd.Flags().Changed("format") {
		rpt, err = rpt.Parse(report.OriginUser, statsOptions.Format)
	} else {
		format := "{{range .}}{{.ID}}\t{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDS}}\t{{.UpTime}}\t{{.AVGCPU}}\n{{end -}}"
		rpt, err = rpt.Parse(report.OriginPodman, format)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		if err := rpt.Execute(headers); err != nil {
			return err
		}
	}
	return rpt.Execute(stats)
}

type containerStats struct {
	define.ContainerStats
}

func (s *containerStats) ID() string {
	if notrunc {
		return s.ContainerID
	}
	return s.ContainerID[0:12]
}

func (s *containerStats) CPUPerc() string {
	return floatToPercentString(s.CPU)
}

func (s *containerStats) AVGCPU() string {
	return floatToPercentString(s.AvgCPU)
}

func (s *containerStats) Up() string {
	return s.UpTime.String()
}

func (s *containerStats) MemPerc() string {
	return floatToPercentString(s.ContainerStats.MemPerc)
}

func (s *containerStats) NetIO() string {
	var netInput uint64
	var netOutput uint64

	for _, net := range s.Network {
		netInput += net.RxBytes
		netOutput += net.TxBytes
	}

	return combineHumanValues(netInput, netOutput)
}

func (s *containerStats) BlockIO() string {
	return combineHumanValues(s.BlockInput, s.BlockOutput)
}

func (s *containerStats) PIDS() string {
	return strconv.FormatUint(s.PIDs, 10)
}

func (s *containerStats) MemUsage() string {
	return combineHumanValues(s.ContainerStats.MemUsage, s.ContainerStats.MemLimit)
}

func (s *containerStats) MemUsageBytes() string {
	return combineBytesValues(s.ContainerStats.MemUsage, s.ContainerStats.MemLimit)
}

func floatToPercentString(f float64) string {
	return fmt.Sprintf("%.2f%%", f)
}

func combineHumanValues(a, b uint64) string {
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func combineBytesValues(a, b uint64) string {
	return fmt.Sprintf("%s / %s", units.BytesSize(float64(a)), units.BytesSize(float64(b)))
}

func outputJSON(stats []containerStats) error {
	type jstat struct {
		Id         string `json:"id"`
		Name       string `json:"name"`
		CPUTime    string `json:"cpu_time"`
		CpuPercent string `json:"cpu_percent"`
		AverageCPU string `json:"avg_cpu"`
		MemUsage   string `json:"mem_usage"`
		MemPerc    string `json:"mem_percent"`
		NetIO      string `json:"net_io"`
		BlockIO    string `json:"block_io"`
		Pids       string `json:"pids"`
	}
	jstats := make([]jstat, 0, len(stats))
	for _, j := range stats {
		jstats = append(jstats, jstat{
			Id:         j.ID(),
			Name:       j.Name,
			CPUTime:    j.Up(),
			CpuPercent: j.CPUPerc(),
			AverageCPU: j.AVGCPU(),
			MemUsage:   j.MemUsage(),
			MemPerc:    j.MemPerc(),
			NetIO:      j.NetIO(),
			BlockIO:    j.BlockIO(),
			Pids:       j.PIDS(),
		})
	}
	b, err := json.MarshalIndent(jstats, "", " ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
