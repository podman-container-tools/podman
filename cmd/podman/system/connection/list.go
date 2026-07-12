package connection

import (
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/system"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	listCmd = &cobra.Command{
		Use:     "list [options]",
		Aliases: []string{"ls"},
		Args:    validate.NoArgs,
		Short:   "List destination for the Podman service(s)",
		Long:    `List destination information for the Podman service(s) in podman configuration`,
		Example: `podman system connection list
# Format as table without TLS info
podman system connection ls
# Format as table with TLS info
podman system connection ls --format=tls
# Format as JSON
podman system connection ls --format=json
# Format as custom go template
podman system connection ls --format='{{range .}}{{.Name}}{{ "\n" }}{{ end }}'`,
		ValidArgsFunction: completion.AutocompleteNone,
		RunE:              list,
		TraverseChildren:  false,
	}
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] [CONTEXT] [CONTEXT...]",
		Short:             "Inspect destination for a Podman service(s)",
		ValidArgsFunction: completion.AutocompleteNone,
		RunE:              inspect,
	}
)

func init() {
	initFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringP("format", "f", "", "Custom Go template for printing connections")
		_ = cmd.RegisterFlagCompletionFunc("format", common.AutocompleteFormat(&config.Connection{}))
		cmd.Flags().BoolP("quiet", "q", false, "Custom Go template for printing connections")
	}

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  system.ContextCmd,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  system.ConnectionCmd,
	})
	initFlags(listCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  system.ContextCmd,
	})
	initFlags(inspectCmd)
}

func list(cmd *cobra.Command, _ []string) error {
	return inspect(cmd, nil)
}

func inspect(cmd *cobra.Command, args []string) error {
	format := cmd.Flag("format").Value.String()
	if format == "" && args != nil {
		format = "json"
	}

	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return err
	}

	cons, err := registry.PodmanConfig().ContainersConfDefaultsRO.GetAllConnections()
	if err != nil {
		return err
	}
	rows := make([]config.Connection, 0, len(cons))
	for _, con := range cons {
		if args != nil && !slices.Contains(args, con.Name) {
			continue
		}

		if quiet {
			fmt.Println(con.Name)
			continue
		}

		rows = append(rows, con)
	}

	if quiet {
		return nil
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	if report.IsJSON(format) {
		buf, err := registry.JSONLibrary().MarshalIndent(rows, "", "    ")
		if err == nil {
			fmt.Println(string(buf))
		}
		return err
	}

	switch format {
	case "tls":
		rpt, err = rpt.Parse(report.OriginPodman,
			"{{range .}}{{.Name}}\t{{.URI}}\t{{.Identity}}\t{{.TLSCA}}\t{{.TLSCert}}\t{{.TLSKey}}\t{{.Default}}\t{{.ReadWrite}}\n{{end -}}")
	case "":
		rpt, err = rpt.Parse(report.OriginPodman,
			"{{range .}}{{.Name}}\t{{.URI}}\t{{.Identity}}\t{{.Default}}\t{{.ReadWrite}}\n{{end -}}")
	default:
		rpt, err = rpt.Parse(report.OriginUser, format)
	}

	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		err = rpt.Execute([]map[string]string{{
			"Default":   "Default",
			"Identity":  "Identity",
			"TLSCA":     "TLSCA",
			"TLSCert":   "TLSCert",
			"TLSKey":    "TLSKey",
			"Name":      "Name",
			"URI":       "URI",
			"ReadWrite": "ReadWrite",
		}})
		if err != nil {
			return err
		}
	}
	return rpt.Execute(rows)
}
