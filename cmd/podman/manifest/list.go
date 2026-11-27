package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	listCmd = &cobra.Command{
		Use:               "list [options]",
		Aliases:           []string{"ls"},
		Short:             "List manifest lists and image indexes in local storage",
		Long:              "List manifest lists and image indexes stored in local container storage.",
		RunE:              list,
		Args:              validate.NoArgs,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman manifest list
podman manifest list --format json
podman manifest ls
podman manifest ls  --format "table {{.ID}} {{.Repository}} {{.Tag}}"
`,
	}

	listFlag = listFlagType{}
)

type listFlagType struct {
	format    string
	noHeading bool
	noTrunc   bool
	quiet     bool
}

// listReporter is one row of human-readable output. One row is emitted per
// repo:tag of a manifest list (or a single <none> row if it has no names).
type listReporter struct {
	Repository string
	Tag        string
	ID         string
	Digest     string
	Created    string
	CreatedAt  string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  manifestCmd,
	})

	flags := listCmd.Flags()

	formatFlagName := "format"
	flags.StringVarP(&listFlag.format, formatFlagName, "f", "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Created}}", "Format the output to a Go template or json")
	_ = listCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&listReporter{}))

	flags.BoolVarP(&listFlag.noHeading, "noheading", "n", false, "Do not print headers")
	flags.BoolVar(&listFlag.noTrunc, "no-trunc", false, "Do not truncate output")
	flags.BoolVarP(&listFlag.quiet, "quiet", "q", false, "Show only manifest list IDs")
}

func list(cmd *cobra.Command, _ []string) error {
	if listFlag.quiet && cmd.Flags().Changed("format") {
		return errors.New("quiet and format flags cannot be used together")
	}

	// Reuse the existing image listing, restricted to manifest lists.
	// This is the same thing as `podman images --filter manifest=true`.
	imgs, err := registry.ImageEngine().List(cmd.Context(), entities.ImageListOptions{
		Filter: []string{"manifest=true"},
	})
	if err != nil {
		return err
	}

	// Newest first, like `podman images`.
	slices.SortFunc(imgs, func(a, b *entities.ImageSummary) int {
		return int(b.Created - a.Created)
	})

	if listFlag.quiet {
		for _, img := range imgs {
			id := strings.TrimPrefix(img.ID, "sha256:")
			if !listFlag.noTrunc && len(id) > 12 {
				id = id[:12]
			}
			fmt.Fprintln(os.Stdout, id)
		}
		return nil
	}

	if report.IsJSON(listFlag.format) {
		b, err := json.MarshalIndent(imgs, "", "    ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, string(b))
		return err
	}

	rows, err := toHumanFormat(imgs)
	if err != nil {
		return err
	}
	return outputTemplate(cmd, rows)
}

func outputTemplate(cmd *cobra.Command, rows []*listReporter) error {
	headers := report.Headers(listReporter{}, map[string]string{
		"ID":        "IMAGE ID",
		"CreatedAt": "CREATED AT",
	})

	rpt := report.New(os.Stdout, cmd.Name())
	defer rpt.Flush()

	var err error
	switch {
	case cmd.Flag("format").Changed:
		rpt, err = rpt.Parse(report.OriginUser, listFlag.format)
	default:
		rpt, err = rpt.Parse(report.OriginPodman, listFlag.format)
	}
	if err != nil {
		return err
	}

	if rpt.RenderHeaders && !listFlag.noHeading {
		if err := rpt.Execute(headers); err != nil {
			return fmt.Errorf("failed to write report column headers: %w", err)
		}
	}
	return rpt.Execute(rows)
}

func toHumanFormat(imgs []*entities.ImageSummary) ([]*listReporter, error) {
	rows := make([]*listReporter, 0, len(imgs))
	for _, img := range imgs {
		created := time.Unix(img.Created, 0)

		id := strings.TrimPrefix(img.ID, "sha256:")
		if !listFlag.noTrunc && len(id) > 12 {
			id = id[:12]
		}

		base := listReporter{
			ID:        id,
			Digest:    img.Digest,
			Created:   units.HumanDuration(time.Since(created)) + " ago",
			CreatedAt: created.Format("2006-01-02 15:04:05 -0700 MST"),
		}

		if len(img.RepoTags) == 0 {
			row := base
			row.Repository, row.Tag = "<none>", "<none>"
			rows = append(rows, &row)
			continue
		}
		for _, rt := range img.RepoTags {
			row := base
			repo, tag, err := utils.TokenRepoTag(rt)
			if err != nil {
				return nil, fmt.Errorf("parsing repository tag: %q: %w", rt, err)
			}
			row.Repository, row.Tag = repo, tag
			rows = append(rows, &row)
		}
	}
	return rows, nil
}
