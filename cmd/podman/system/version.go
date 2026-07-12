package system

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	versionCommand = &cobra.Command{
		Use:               "version [options]",
		Args:              validate.NoArgs,
		Short:             "Display the Podman version information",
		RunE:              version,
		ValidArgsFunction: completion.AutocompleteNone,
		Annotations: map[string]string{
			registry.ParentNSRequired: "",
		},
	}
	versionFormat string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: versionCommand,
	})
	flags := versionCommand.Flags()

	formatFlagName := "format"
	flags.StringVarP(&versionFormat, formatFlagName, "f", "", "Change the output format to JSON or a Go template")
	_ = versionCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.SystemVersionReport{}))
}

func version(cmd *cobra.Command, _ []string) error {
	versions, err := registry.ContainerEngine().Version(registry.Context())
	if err != nil {
		return err
	}
	return PrintVersion(cmd, versions)
}

func PrintVersion(cmd *cobra.Command, versions *entities.SystemVersionReport) error {
	var err error

	if report.IsJSON(versionFormat) {
		s, err := json.MarshalToString(versions)
		if err != nil {
			return err
		}
		fmt.Println(s)
		return nil
	}

	if cmd.Flag("format").Changed {
		rpt := report.New(os.Stdout, cmd.Name())
		defer rpt.Flush()

		// Use OriginUnknown so it does not add an extra range since it
		// will only be called for a single element and not a slice.
		rpt, err = rpt.Parse(report.OriginUnknown, versionFormat)
		if err != nil {
			return err
		}
		if err := rpt.Execute(versions); err != nil {
			// only log at debug since we fall back to the client only template
			logrus.Debugf("Failed to execute template: %v", err)
			// On Failure, assume user is using older version of podman version --format and check client
			versionFormat = strings.ReplaceAll(versionFormat, ".Server.", ".")
			rpt, err := rpt.Parse(report.OriginUnknown, versionFormat)
			if err != nil {
				return err
			}
			if err := rpt.Execute(versions.Client); err != nil {
				return err
			}
		}
		return nil
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()
	rpt, err = rpt.Parse(report.OriginPodman, versionTemplate)
	if err != nil {
		return err
	}
	return rpt.Execute(versions)
}

const versionTemplate = `{{with .Client -}}
Client:\tPodman Engine
Version:\t{{.Version}}
API Version:\t{{.APIVersion}}
Go Version:\t{{.GoVersion}}
{{if .GitCommit -}}Git Commit:\t{{.GitCommit}}\n{{end -}}
Built:\t{{.BuiltTime}}
{{if .BuildOrigin -}}Build Origin:\t{{.BuildOrigin}}\n{{end -}}
OS/Arch:\t{{.OsArch}}
{{- end}}

{{- if .Server }}{{with .Server}}

Server:\tPodman Engine
Version:\t{{.Version}}
API Version:\t{{.APIVersion}}
Go Version:\t{{.GoVersion}}
{{if .GitCommit -}}Git Commit:\t{{.GitCommit}}\n{{end -}}
Built:\t{{.BuiltTime}}
{{if .BuildOrigin -}}Build Origin:\t{{.BuildOrigin}}\n{{end -}}
OS/Arch:\t{{.OsArch}}
{{- end}}{{- end}}
`
