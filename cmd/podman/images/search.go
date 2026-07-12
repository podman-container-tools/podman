package images

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// searchOptionsWrapper wraps entities.ImagePullOptions and prevents leaking
// CLI-only fields into the API types.
type searchOptionsWrapper struct {
	entities.ImageSearchOptions
	// CLI only flags
	Compatible     bool // Docker compat
	CredentialsCLI string
	TLSVerifyCLI   bool   // Used to convert to an optional bool later
	Format         string // For go templating
	NoTrunc        bool
}

// listEntryTag is a utility structure used for json serialization.
type listEntryTag struct {
	Name string
	Tags []string
}

var (
	searchOptions     = searchOptionsWrapper{}
	searchDescription = `Search registries for a given image. Can search all the default registries or a specific registry.

	Users can limit the number of results, and filter the output based on certain conditions.`

	searchCmd = &cobra.Command{
		Use:               "search [options] TERM",
		Short:             "Search registry for image",
		Long:              searchDescription,
		RunE:              imageSearch,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman search --filter=is-official --limit 3 alpine
podman search registry.fedoraproject.org/  # only works with v2 registries
podman search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}

	imageSearchCmd = &cobra.Command{
		Use:               searchCmd.Use,
		Short:             searchCmd.Short,
		Long:              searchCmd.Long,
		RunE:              searchCmd.RunE,
		Args:              searchCmd.Args,
		ValidArgsFunction: searchCmd.ValidArgsFunction,
		Example: `podman image search --filter=is-official --limit 3 alpine
podman image search registry.fedoraproject.org/  # only works with v2 registries
podman image search --format "table {{.Index}} {{.Name}}" registry.fedoraproject.org/fedora`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: searchCmd,
	})
	searchFlags(searchCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageSearchCmd,
		Parent:  imageCmd,
	})
	searchFlags(imageSearchCmd)
}

// searchFlags set the flags for the pull command.
func searchFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	filterFlagName := "filter"
	flags.StringArrayVarP(&searchOptions.Filters, filterFlagName, "f", []string{}, "Filter output based on conditions provided (default [])")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteImageSearchFilters)

	formatFlagName := "format"
	flags.StringVar(&searchOptions.Format, formatFlagName, "", "Change the output format to JSON or a Go template")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.ImageSearchReport{}))

	limitFlagName := "limit"
	flags.IntVar(&searchOptions.Limit, limitFlagName, 0, "Limit the number of results")
	_ = cmd.RegisterFlagCompletionFunc(limitFlagName, completion.AutocompleteNone)

	flags.BoolVar(&searchOptions.NoTrunc, "no-trunc", false, "Do not truncate the output")
	flags.BoolVar(&searchOptions.Compatible, "compatible", false, "List stars, official and automated columns (Docker compatibility)")

	authfileFlagName := "authfile"
	flags.StringVar(&searchOptions.Authfile, authfileFlagName, auth.GetDefaultAuthFile(), "Path of the authentication file. Use REGISTRY_AUTH_FILE environment variable to override")
	_ = cmd.RegisterFlagCompletionFunc(authfileFlagName, completion.AutocompleteDefault)

	credsFlagName := "creds"
	flags.StringVar(&searchOptions.CredentialsCLI, credsFlagName, "", "`Credentials` (USERNAME:PASSWORD) to use for authenticating to a registry")
	_ = cmd.RegisterFlagCompletionFunc(credsFlagName, completion.AutocompleteNone)

	flags.BoolVar(&searchOptions.TLSVerifyCLI, "tls-verify", true, "Require HTTPS and verify certificates when contacting registries")
	flags.BoolVar(&searchOptions.ListTags, "list-tags", false, "List the tags of the input registry")

	if !registry.IsRemote() {
		certDirFlagName := "cert-dir"
		flags.StringVar(&searchOptions.CertDir, certDirFlagName, "", "`Pathname` of a directory containing TLS certificates and keys")
		_ = cmd.RegisterFlagCompletionFunc(certDirFlagName, completion.AutocompleteDefault)
	}
}

// imageSearch implements the command for searching images.
func imageSearch(cmd *cobra.Command, args []string) error {
	var searchTerm string
	switch len(args) {
	case 1:
		searchTerm = args[0]
	default:
		return errors.New("search requires exactly one argument")
	}

	if searchOptions.ListTags && len(searchOptions.Filters) != 0 {
		return errors.New("filters are not applicable to list tags result")
	}

	// TLS verification in c/image is controlled via a `types.OptionalBool`
	// which allows for distinguishing among set-true, set-false, unspecified
	// which is important to implement a sane way of dealing with defaults of
	// boolean CLI flags.
	if cmd.Flags().Changed("tls-verify") {
		searchOptions.SkipTLSVerify = types.NewOptionalBool(!searchOptions.TLSVerifyCLI)
	}

	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(searchOptions.Authfile); err != nil {
			return err
		}
	}

	if searchOptions.CredentialsCLI != "" {
		creds, err := util.ParseRegistryCreds(searchOptions.CredentialsCLI)
		if err != nil {
			return err
		}
		searchOptions.Username = creds.Username
		searchOptions.Password = creds.Password
	}

	searchReport, err := registry.ImageEngine().Search(registry.Context(), searchTerm, searchOptions.ImageSearchOptions)
	if err != nil {
		return err
	}
	if len(searchReport) == 0 {
		return nil
	}

	isJSON := report.IsJSON(searchOptions.Format)
	for i, element := range searchReport {
		d := strings.ReplaceAll(element.Description, "\n", " ")
		if len(d) > 44 && (!searchOptions.NoTrunc && !isJSON) {
			d = strings.TrimSpace(d[:44]) + "..."
		}
		searchReport[i].Description = d
	}

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	switch {
	case searchOptions.ListTags:
		if len(searchOptions.Filters) != 0 {
			return errors.New("filters are not applicable to list tags result")
		}
		if isJSON {
			listTagsEntries := buildListTagsJSON(searchReport)
			return printArbitraryJSON(listTagsEntries)
		}
		if cmd.Flags().Changed("format") {
			rpt, err = rpt.Parse(report.OriginUser, searchOptions.Format)
		} else {
			rpt, err = rpt.Parse(report.OriginPodman, "{{range .}}{{.Name}}\t{{.Tag}}\n{{end -}}")
		}
	case isJSON:
		return printArbitraryJSON(searchReport)
	case cmd.Flags().Changed("format"):
		rpt, err = rpt.Parse(report.OriginUser, searchOptions.Format)
	default:
		row := "{{.Name}}\t{{.Description}}"
		if searchOptions.Compatible {
			row += "\t{{.Stars}}\t{{.Official}}\t{{.Automated}}"
		}
		row = "{{range . }}" + row + "\n{{end -}}"
		rpt, err = rpt.Parse(report.OriginPodman, row)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders {
		hdrs := report.Headers(entities.ImageSearchReport{}, nil)
		if err := rpt.Execute(hdrs); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(searchReport)
}

func printArbitraryJSON(v any) error {
	prettyJSON, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(prettyJSON))
	return nil
}

func buildListTagsJSON(searchReport []entities.ImageSearchReport) []listEntryTag {
	entries := make([]listEntryTag, 0)

ReportLoop:
	for _, report := range searchReport {
		for idx, entry := range entries {
			if entry.Name == report.Name {
				entries[idx].Tags = append(entries[idx].Tags, report.Tag)
				continue ReportLoop
			}
		}
		newElem := listEntryTag{
			report.Name,
			[]string{report.Tag},
		}

		entries = append(entries, newElem)
	}
	return entries
}
