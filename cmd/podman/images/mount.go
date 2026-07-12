package images

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	mountDescription = `podman image mount
    Lists all mounted images mount points if no images is specified

  podman image mount IMAGE-NAME-OR-ID
    Mounts the specified image and prints the mountpoint
`

	mountCommand = &cobra.Command{
		Annotations: map[string]string{
			registry.UnshareNSRequired: "",
			registry.ParentNSRequired:  "",
			registry.EngineMode:        registry.ABIMode,
		},
		Use:               "mount [options] [IMAGE...]",
		Short:             "Mount an image's root filesystem",
		Long:              mountDescription,
		RunE:              mount,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image mount imgID
podman image mount imgID1 imgID2 imgID3
podman image mount
podman image mount --all`,
	}
)

var mountOpts entities.ImageMountOptions

func mountFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&mountOpts.All, "all", "a", false, "Mount all images")

	formatFlagName := "format"
	flags.StringVar(&mountOpts.Format, formatFlagName, "", "Print the mounted images in specified format (json)")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: mountCommand,
		Parent:  imageCmd,
	})
	mountFlags(mountCommand)
}

func mount(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && mountOpts.All {
		return errors.New("when using the --all switch, you may not pass any image names or IDs")
	}

	reports, err := registry.ImageEngine().Mount(registry.Context(), args, mountOpts)
	if err != nil {
		return err
	}

	if len(args) == 1 && mountOpts.Format == "" && !mountOpts.All {
		if len(reports) != 1 {
			return fmt.Errorf("internal error: expected 1 report but got %d", len(reports))
		}
		fmt.Println(reports[0].Path)
		return nil
	}

	switch {
	case report.IsJSON(mountOpts.Format):
		return printJSON(reports)
	case mountOpts.Format == "":
		break // see default format below
	default:
		return fmt.Errorf("unknown --format argument: %q", mountOpts.Format)
	}

	mrs := make([]mountReporter, 0, len(reports))
	for _, r := range reports {
		mrs = append(mrs, mountReporter{r})
	}

	rpt, err := report.New(os.Stdout, cmd.Name()).Parse(report.OriginPodman, "{{range . }}{{.ID}}\t{{.Path}}\n{{end -}}")
	if err != nil {
		return err
	}
	defer rpt.Flush()
	return rpt.Execute(mrs)
}

func printJSON(reports []*entities.ImageMountReport) error {
	type jreport struct {
		ID           string `json:"id"`
		Names        []string
		Repositories []string
		Mountpoint   string `json:"mountpoint"`
	}
	jreports := make([]jreport, 0, len(reports))

	for _, r := range reports {
		jreports = append(jreports, jreport{
			ID:           r.Id,
			Names:        []string{r.Name},
			Repositories: r.Repositories,
			Mountpoint:   r.Path,
		})
	}
	b, err := json.MarshalIndent(jreports, "", " ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

type mountReporter struct {
	*entities.ImageMountReport
}

func (m mountReporter) ID() string {
	if len(m.Repositories) > 0 {
		return m.Repositories[0]
	}
	return m.Id
}
