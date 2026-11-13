package manifest

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	inspectOptions entities.ManifestInspectOptions
	tlsVerifyCLI   bool
	inspectCmd     = &cobra.Command{
		Use:               "inspect [options] IMAGE",
		Short:             "Display the contents of a manifest list or image index",
		Long:              "Display the contents of a manifest list or image index.",
		RunE:              inspect,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman manifest inspect localhost/list
podman manifest inspect --format "{{.MediaType}}" localhost/list"
podman manifest inspect --format '{{range .Manifests}}{{.Platform.OS}}/{{.Platform.Architecture}} {{.Digest}}\n{{end}}' localhost/list`,
		Args: cobra.ExactArgs(1),
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  manifestCmd,
	})
	flags := inspectCmd.Flags()

	authfileFlagName := "authfile"
	flags.StringVar(&inspectOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = inspectCmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	flags.BoolP("verbose", "v", false, "Added for Docker compatibility")
	_ = flags.MarkHidden("verbose")

	flags.BoolVar(&tlsVerifyCLI, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")

	flags.Bool("insecure", false, "Purely for Docker compatibility")
	_ = flags.MarkHidden("insecure")

	formatFlagName := "format"
	flags.StringVarP(&inspectOptions.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteDefault)

}

func inspect(cmd *cobra.Command, args []string) error {
	var errs []error
	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(inspectOptions.Authfile); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("tls-verify") {
		inspectOptions.SkipTLSVerify = types.NewOptionalBool(!tlsVerifyCLI)
	} else if cmd.Flags().Changed("insecure") {
		insecure, _ := cmd.Flags().GetBool("insecure")
		inspectOptions.SkipTLSVerify = types.NewOptionalBool(insecure)
	}

	list, err := registry.ImageEngine().ManifestInspect(cmd.Context(), args[0], inspectOptions)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(inspectOptions.Format) || inspectOptions.Format == "":
		prettyJSON, err := json.MarshalIndent(list, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(prettyJSON))
		return nil
	default:
		// Landing here implies user has given a custom --format
		format := report.NormalizeFormat(inspectOptions.Format)
		rpt, err := report.New(os.Stdout, "manifest inspect").Parse(report.OriginUser, format)
		if err != nil {
			return err
		}
		defer rpt.Flush()

		err = rpt.Execute([]any{list})
	}
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		if len(errs) > 1 {
			for _, err := range errs[1:] {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
		}
		return errs[0]
	}
	return nil
}
