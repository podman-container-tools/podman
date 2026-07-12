package system

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	dfSystemDescription = `
	podman system df

	Show podman disk usage
	`
	dfSystemCommand = &cobra.Command{
		Use:               "df [options]",
		Args:              validate.NoArgs,
		Short:             "Show podman disk usage",
		Long:              dfSystemDescription,
		RunE:              df,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var dfOptions entities.SystemDfOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: dfSystemCommand,
		Parent:  systemCmd,
	})
	flags := dfSystemCommand.Flags()
	flags.BoolVarP(&dfOptions.Verbose, "verbose", "v", false, "Show detailed information on disk usage")

	formatFlagName := "format"
	flags.StringVar(&dfOptions.Format, formatFlagName, "", "Pretty-print images using a Go template")
	_ = dfSystemCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&dfSummary{}))
}

func df(cmd *cobra.Command, _ []string) error {
	reports, err := registry.ContainerEngine().SystemDf(registry.Context(), dfOptions)
	if err != nil {
		return err
	}

	if dfOptions.Format != "" && dfOptions.Verbose {
		return errors.New("cannot combine --format and --verbose flags")
	}

	if dfOptions.Verbose {
		return printVerbose(cmd, reports)
	}
	return printSummary(cmd, reports)
}

func printSummary(cmd *cobra.Command, reports *entities.SystemDfReport) error {
	var (
		active int
		used   int64
	)

	visitedImages := make(map[string]bool)
	for _, i := range reports.Images {
		if _, ok := visitedImages[i.ImageID]; ok {
			continue
		}
		visitedImages[i.ImageID] = true
		if i.Containers > 0 {
			active++
			used += i.UniqueSize
		}
	}

	imageSummary := dfSummary{
		Type:           "Images",
		Total:          len(reports.Images),
		Active:         active,
		RawSize:        reports.ImagesSize,        // The "raw" size is the sum of all layer sizes
		RawReclaimable: reports.ImagesSize - used, // We can reclaim the date of "unused" images (i.e., the ones without containers)
	}

	// Containers
	var (
		conActive               int
		conSize, conReclaimable int64
	)
	for _, c := range reports.Containers {
		if c.Status == "running" {
			conActive++
		} else {
			conReclaimable += c.RWSize
		}
		conSize += c.RWSize
	}
	containerSummary := dfSummary{
		Type:           "Containers",
		Total:          len(reports.Containers),
		Active:         conActive,
		RawSize:        conSize,
		RawReclaimable: conReclaimable,
	}

	// Volumes
	var (
		activeVolumes                   int
		volumesSize, volumesReclaimable int64
	)

	for _, v := range reports.Volumes {
		activeVolumes += v.Links
		volumesSize += v.Size
		volumesReclaimable += v.ReclaimableSize
	}
	volumeSummary := dfSummary{
		Type:           "Local Volumes",
		Total:          len(reports.Volumes),
		Active:         activeVolumes,
		RawSize:        volumesSize,
		RawReclaimable: volumesReclaimable,
	}
	dfSummaries := []*dfSummary{
		&imageSummary,
		&containerSummary,
		&volumeSummary,
	}

	// need to give un-exported fields
	hdrs := report.Headers(dfSummary{}, map[string]string{
		"Size":        "SIZE",
		"Reclaimable": "RECLAIMABLE",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	if cmd.Flags().Changed("format") {
		if report.IsJSON(dfOptions.Format) {
			return printJSON(dfSummaries)
		}
		rpt, err = rpt.Parse(report.OriginUser, dfOptions.Format)
	} else {
		row := "{{range . }}{{.Type}}\t{{.Total}}\t{{.Active}}\t{{.Size}}\t{{.Reclaimable}}\n{{end -}}"
		rpt, err = rpt.Parse(report.OriginPodman, row)
	}
	if err != nil {
		return err
	}
	return writeTemplate(rpt, hdrs, dfSummaries)
}

func printJSON(data []*dfSummary) error {
	bytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}

func printVerbose(cmd *cobra.Command, reports *entities.SystemDfReport) error {
	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	fmt.Fprint(rpt.Writer(), "Images space usage:\n\n")
	// convert to dfImage for output
	dfImages := make([]*dfImage, 0, len(reports.Images))
	for _, d := range reports.Images {
		dfImages = append(dfImages, &dfImage{SystemDfImageReport: d})
	}
	hdrs := report.Headers(entities.SystemDfImageReport{}, map[string]string{
		"ImageID":    "IMAGE ID",
		"SharedSize": "SHARED SIZE",
		"UniqueSize": "UNIQUE SIZE",
	})
	imageRow := "{{range .}}{{.Repository}}\t{{.Tag}}\t{{.ImageID}}\t{{.Created}}\t{{.Size}}\t{{.SharedSize}}\t{{.UniqueSize}}\t{{.Containers}}\n{{end -}}"
	rpt, err := rpt.Parse(report.OriginPodman, imageRow)
	if err != nil {
		return err
	}
	if err := writeTemplate(rpt, hdrs, dfImages); err != nil {
		return err
	}

	fmt.Fprint(rpt.Writer(), "\nContainers space usage:\n\n")
	// convert to dfContainers for output
	dfContainers := make([]*dfContainer, 0, len(reports.Containers))
	for _, d := range reports.Containers {
		dfContainers = append(dfContainers, &dfContainer{SystemDfContainerReport: d})
	}
	hdrs = report.Headers(entities.SystemDfContainerReport{}, map[string]string{
		"ContainerID":  "CONTAINER ID",
		"LocalVolumes": "LOCAL VOLUMES",
		"RWSize":       "SIZE",
	})
	containerRow := "{{range .}}{{.ContainerID}}\t{{.Image}}\t{{.Command}}\t{{.LocalVolumes}}\t{{.RWSize}}\t{{.Created}}\t{{.Status}}\t{{.Names}}\n{{end -}}"
	rpt, err = rpt.Parse(report.OriginPodman, containerRow)
	if err != nil {
		return err
	}
	if err := writeTemplate(rpt, hdrs, dfContainers); err != nil {
		return err
	}

	fmt.Fprint(rpt.Writer(), "\nLocal Volumes space usage:\n\n")
	dfVolumes := make([]*dfVolume, 0, len(reports.Volumes))
	// convert to dfVolume for output
	for _, d := range reports.Volumes {
		dfVolumes = append(dfVolumes, &dfVolume{SystemDfVolumeReport: d})
	}
	hdrs = report.Headers(entities.SystemDfVolumeReport{}, map[string]string{
		"VolumeName": "VOLUME NAME",
	})
	volumeRow := "{{range .}}{{.VolumeName}}\t{{.Links}}\t{{.Size}}\n{{end -}}"
	rpt, err = rpt.Parse(report.OriginPodman, volumeRow)
	if err != nil {
		return err
	}
	return writeTemplate(rpt, hdrs, dfVolumes)
}

func writeTemplate(rpt *report.Formatter, hdrs []map[string]string, output any) error {
	if rpt.RenderHeaders {
		if err := rpt.Execute(hdrs); err != nil {
			return err
		}
	}
	return rpt.Execute(output)
}

type dfImage struct {
	*entities.SystemDfImageReport
}

func (d *dfImage) ImageID() string {
	return d.SystemDfImageReport.ImageID[0:12]
}

func (d *dfImage) Created() string {
	return units.HumanDuration(time.Since(d.SystemDfImageReport.Created))
}

func (d *dfImage) Size() string {
	return units.HumanSize(float64(d.SystemDfImageReport.Size))
}

func (d *dfImage) SharedSize() string {
	return units.HumanSize(float64(d.SystemDfImageReport.SharedSize))
}

func (d *dfImage) UniqueSize() string {
	return units.HumanSize(float64(d.SystemDfImageReport.UniqueSize))
}

type dfContainer struct {
	*entities.SystemDfContainerReport
}

func (d *dfContainer) ContainerID() string {
	return d.SystemDfContainerReport.ContainerID[0:12]
}

func (d *dfContainer) Image() string {
	if len(d.SystemDfContainerReport.Image) >= 12 {
		return d.SystemDfContainerReport.Image[0:12]
	}
	return ""
}

func (d *dfContainer) Command() string {
	return strings.Join(d.SystemDfContainerReport.Command, " ")
}

func (d *dfContainer) RWSize() string {
	return units.HumanSize(float64(d.SystemDfContainerReport.RWSize))
}

func (d *dfContainer) Created() string {
	return units.HumanDuration(time.Since(d.SystemDfContainerReport.Created))
}

type dfVolume struct {
	*entities.SystemDfVolumeReport
}

func (d *dfVolume) Size() string {
	return units.HumanSize(float64(d.SystemDfVolumeReport.Size))
}

type dfSummary struct {
	Type           string
	Total          int
	Active         int
	RawSize        int64
	RawReclaimable int64
}

func (d *dfSummary) Size() string {
	return units.HumanSize(float64(d.RawSize))
}

func (d *dfSummary) Reclaimable() string {
	percent := 0
	// make sure to check this to prevent div by zero problems
	if d.RawSize > 0 {
		percent = int(math.Round(float64(d.RawReclaimable) / float64(d.RawSize) * float64(100)))
	}
	return fmt.Sprintf("%s (%d%%)", units.HumanSize(float64(d.RawReclaimable)), percent)
}

func (d *dfSummary) MarshalJSON() ([]byte, error) {
	// need to create a new type here to prevent infinite recursion in MarshalJSON() call
	type rawDf dfSummary

	return json.Marshal(struct {
		rawDf
		// fields for docker compat https://github.com/containers/podman/issues/16902
		TotalCount  int
		Size        string
		Reclaimable string
	}{rawDf(*d), d.Total, d.Size(), d.Reclaimable()})
}
