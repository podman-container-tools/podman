package containers

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	mountDescription = `podman mount
    Lists all mounted containers mount points if no container is specified

  podman mount CONTAINER-NAME-OR-ID
    Mounts the specified container and outputs the mountpoint
`

	mountCommand = &cobra.Command{
		Annotations: map[string]string{
			registry.UnshareNSRequired: "",
			registry.ParentNSRequired:  "",
			registry.EngineMode:        registry.ABIMode,
		},
		Use:   "mount [options] [CONTAINER...]",
		Short: "Mount a working container's root filesystem",
		Long:  mountDescription,
		RunE:  mount,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, true, "")
		},
		ValidArgsFunction: common.AutocompleteContainers,
	}

	containerMountCommand = &cobra.Command{
		Annotations:       mountCommand.Annotations,
		Use:               mountCommand.Use,
		Short:             mountCommand.Short,
		Long:              mountCommand.Long,
		RunE:              mountCommand.RunE,
		Args:              mountCommand.Args,
		ValidArgsFunction: mountCommand.ValidArgsFunction,
	}
)

var mountOpts entities.ContainerMountOptions

func mountFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&mountOpts.All, "all", "a", false, "Mount all containers")

	formatFlagName := "format"
	flags.StringVar(&mountOpts.Format, formatFlagName, "", "Print the mounted containers in specified format (json)")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil))

	flags.BoolVar(&mountOpts.NoTruncate, "no-trunc", false, "Do not truncate output")
	flags.SetNormalizeFunc(utils.AliasFlags)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: mountCommand,
	})
	mountFlags(mountCommand)
	validate.AddLatestFlag(mountCommand, &mountOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerMountCommand,
		Parent:  containerCmd,
	})
	mountFlags(containerMountCommand)
	validate.AddLatestFlag(containerMountCommand, &mountOpts.Latest)
}

func mount(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && mountOpts.Latest {
		return errors.New("--latest and containers cannot be used together")
	}
	args = utils.RemoveSlash(args)

	reports, err := registry.ContainerEngine().ContainerMount(registry.Context(), args, mountOpts)
	if err != nil {
		return err
	}

	if len(args) > 0 || mountOpts.Latest || mountOpts.All {
		var errs utils.OutputErrors
		for _, r := range reports {
			if r.Err == nil {
				fmt.Println(r.Path)
				continue
			}
			errs = append(errs, r.Err)
		}
		return errs.PrintErrors()
	}

	switch {
	case report.IsJSON(mountOpts.Format):
		return printJSON(reports)
	case mountOpts.Format == "":
		break // print defaults
	default:
		return fmt.Errorf("unknown --format argument: %q", mountOpts.Format)
	}

	mrs := make([]mountReporter, 0, len(reports))
	for _, r := range reports {
		mrs = append(mrs, mountReporter{r})
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	rpt, err = rpt.Parse(report.OriginPodman, "{{range . }}{{.ID}}\t{{.Path}}\n{{end -}}")
	if err != nil {
		return err
	}
	return rpt.Execute(mrs)
}

func printJSON(reports []*entities.ContainerMountReport) error {
	type jreport struct {
		ID         string `json:"id"`
		Names      []string
		Mountpoint string `json:"mountpoint"`
	}
	jreports := make([]jreport, 0, len(reports))

	for _, r := range reports {
		jreports = append(jreports, jreport{
			ID:         r.Id,
			Names:      []string{r.Name},
			Mountpoint: r.Path,
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
	*entities.ContainerMountReport
}

func (m mountReporter) ID() string {
	if mountOpts.NoTruncate {
		return m.Id
	}
	return m.Id[0:12]
}
