package images

import (
	"fmt"
	"os"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	long = `Displays the history of an image.

  The information can be printed out in an easy to read, or user specified format, and can be truncated.`

	// podman _history_
	historyCmd = &cobra.Command{
		Use:               "history [options] IMAGE",
		Short:             "Show history of a specified image",
		Long:              long,
		Args:              cobra.ExactArgs(1),
		RunE:              history,
		ValidArgsFunction: common.AutocompleteImages,
		Example:           "podman history quay.io/fedora/fedora",
	}

	imageHistoryCmd = &cobra.Command{
		Args:              historyCmd.Args,
		Use:               historyCmd.Use,
		Short:             historyCmd.Short,
		Long:              historyCmd.Long,
		ValidArgsFunction: historyCmd.ValidArgsFunction,
		RunE:              historyCmd.RunE,
		Example:           `podman image history quay.io/fedora/fedora`,
	}

	opts = struct {
		human   bool
		noTrunc bool
		quiet   bool
		format  string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: historyCmd,
	})
	historyFlags(historyCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageHistoryCmd,
		Parent:  imageCmd,
	})
	historyFlags(imageHistoryCmd)
}

func historyFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	formatFlagName := "format"
	flags.StringVar(&opts.format, formatFlagName, "", "Change the output to JSON or a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&historyReporter{}))

	flags.BoolVarP(&opts.human, "human", "H", true, "Display sizes and dates in human readable format")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "Do not truncate the output")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Display the numeric IDs only")
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func history(cmd *cobra.Command, args []string) error {
	results, err := registry.ImageEngine().History(registry.Context(), args[0], entities.ImageHistoryOptions{})
	if err != nil {
		return err
	}

	if report.IsJSON(opts.format) {
		var err error
		if len(results.Layers) == 0 {
			_, err = fmt.Fprintf(os.Stdout, "[]\n")
		} else {
			// ah-hoc change to "Created": type and format
			type layer struct {
				entities.ImageHistoryLayer
				Created string `json:"Created"`
			}

			layers := make([]layer, len(results.Layers))
			for i, l := range results.Layers {
				layers[i].ImageHistoryLayer = l
				layers[i].Created = l.Created.Format(time.RFC3339)
			}
			enc := json.NewEncoder(os.Stdout)
			err = enc.Encode(layers)
		}
		return err
	}

	hr := make([]historyReporter, 0, len(results.Layers))
	for _, l := range results.Layers {
		hr = append(hr, historyReporter{l})
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	switch {
	case opts.quiet:
		rpt, err = rpt.Parse(report.OriginUser, "{{range .}}{{.ID}}\n{{end -}}")
	case cmd.Flags().Changed("format"):
		rpt, err = rpt.Parse(report.OriginUser, cmd.Flag("format").Value.String())
	default:
		format := "{{range .}}{{.ID}}\t{{.Created}}\t{{.CreatedBy}}\t{{.Size}}\t{{.Comment}}\n{{end -}}"
		rpt, err = rpt.Parse(report.OriginPodman, format)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		hdrs := report.Headers(historyReporter{}, map[string]string{
			"CreatedBy": "CREATED BY",
		})

		if err := rpt.Execute(hdrs); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(hr)
}

type historyReporter struct {
	entities.ImageHistoryLayer
}

func (h historyReporter) Created() string {
	if opts.human {
		return units.HumanDuration(time.Since(h.ImageHistoryLayer.Created)) + " ago"
	}
	return h.ImageHistoryLayer.Created.Format(time.RFC3339)
}

func (h historyReporter) Size() string {
	return units.HumanSizeWithPrecision(float64(h.ImageHistoryLayer.Size), 3)
}

func (h historyReporter) CreatedBy() string {
	if !opts.noTrunc && len(h.ImageHistoryLayer.CreatedBy) > 45 {
		return h.ImageHistoryLayer.CreatedBy[:45-3] + "..."
	}
	return h.ImageHistoryLayer.CreatedBy
}

func (h historyReporter) ID() string {
	if !opts.noTrunc && len(h.ImageHistoryLayer.ID) >= 12 {
		return h.ImageHistoryLayer.ID[0:12]
	}
	return h.ImageHistoryLayer.ID
}

func (h historyReporter) CreatedAt() string {
	return time.Unix(h.ImageHistoryLayer.Created.Unix(), 0).Format(time.RFC3339)
}

func (h historyReporter) CreatedSince() string {
	return h.Created()
}
