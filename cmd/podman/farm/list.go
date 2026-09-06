package farm

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	farmLsDescription = `podman farm ls

List all available farms. The output of the farms can be filtered
and the output format can be changed to JSON or a user specified Go template.`
	lsCommand = &cobra.Command{
		Use:                "list [options]",
		Aliases:            []string{"ls"},
		Args:               validate.NoArgs,
		Short:              "List all existing farms",
		Long:               farmLsDescription,
		PersistentPreRunE:  validate.NoOp,
		RunE:               list,
		PersistentPostRunE: validate.NoOp,
		ValidArgsFunction:  completion.AutocompleteNone,
	}

	// Temporary struct to hold cli values.
	lsOpts = struct {
		Format string
		Quiet  bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: lsCommand,
		Parent:  farmCmd,
	})
	flags := lsCommand.Flags()

	formatFlagName := "format"
	flags.StringVar(&lsOpts.Format, formatFlagName, "", "Format farm output using Go template")
	_ = lsCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&config.Farm{}))

	flags.BoolVarP(&lsOpts.Quiet, "quiet", "q", false, "Print farm names only")
	flags.BoolP("noheading", "n", false, "Do not print headers")
}

func list(cmd *cobra.Command, args []string) error {
	if lsOpts.Quiet && cmd.Flag("format").Changed {
		return errors.New("quiet and format flags cannot be used together")
	}
	noHeading, _ := cmd.Flags().GetBool("noheading")
	format := lsOpts.Format
	if format == "" && len(args) > 0 {
		format = "json"
	}

	farms, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetAllFarms()
	if err != nil {
		return err
	}

	slices.SortFunc(farms, func(a, b config.Farm) int {
		return cmp.Compare(a.Name, b.Name)
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	if report.IsJSON(format) {
		buf, err := registry.JSONLibrary().MarshalIndent(farms, "", "    ")
		if err == nil {
			fmt.Println(string(buf))
		}
		return err
	}

	switch {
	case format != "":
		rpt, err = rpt.Parse(report.OriginUser, format)
	case lsOpts.Quiet:
		rpt, err = rpt.Parse(report.OriginUser, "{{range .}}{{.Name}}\n{{end -}}")
	default:
		rpt, err = rpt.Parse(report.OriginPodman,
			"{{range .}}{{.Name}}\t{{.Connections}}\t{{.Default}}\t{{.ReadWrite}}\n{{end -}}")
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders && !noHeading {
		err = rpt.Execute([]map[string]string{{
			"Default":     "Default",
			"Connections": "Connections",
			"Name":        "Name",
			"ReadWrite":   "ReadWrite",
		}})
		if err != nil {
			return err
		}
	}

	return rpt.Execute(farms)
}
