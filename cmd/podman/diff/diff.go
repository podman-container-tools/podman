package diff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/storage/pkg/archive"
)

func Diff(_ *cobra.Command, args []string, options entities.DiffOptions) error {
	results, err := registry.ContainerEngine().Diff(registry.Context(), args, options)
	if err != nil {
		return err
	}

	switch {
	case report.IsJSON(options.Format):
		return changesToJSON(results)
	case options.Format == "":
		return changesToTable(results)
	default:
		return errors.New("only supported value for '--format' is 'json'")
	}
}

type ChangesReportJSON struct {
	Changed []string `json:"changed,omitempty"`
	Added   []string `json:"added,omitempty"`
	Deleted []string `json:"deleted,omitempty"`
}

func changesToJSON(diffs *entities.DiffReport) error {
	body := ChangesReportJSON{}
	for _, row := range diffs.Changes {
		switch row.Kind {
		case archive.ChangeAdd:
			body.Added = append(body.Added, row.Path)
		case archive.ChangeDelete:
			body.Deleted = append(body.Deleted, row.Path)
		case archive.ChangeModify:
			body.Changed = append(body.Changed, row.Path)
		default:
			return fmt.Errorf("output kind %q not recognized", row.Kind)
		}
	}

	// Pull in configured json library
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "     ")
	return enc.Encode(body)
}

func changesToTable(diffs *entities.DiffReport) error {
	for _, row := range diffs.Changes {
		fmt.Fprintln(os.Stdout, row.String())
	}
	return nil
}

// ValidateContainerDiffArgs used to validate a nameOrId was provided or the "--latest" flag
func ValidateContainerDiffArgs(cmd *cobra.Command, args []string) error {
	given, _ := cmd.Flags().GetBool("latest")
	if len(args) > 0 && !given {
		return cobra.RangeArgs(1, 2)(cmd, args)
	}
	if len(args) > 0 && given {
		return errors.New("--latest and containers cannot be used together")
	}
	if len(args) == 0 && !given {
		if registry.IsRemote() {
			return fmt.Errorf("%q requires a name or id", cmd.CommandPath())
		}
		return fmt.Errorf("%q requires a name, id, or the \"--latest\" flag", cmd.CommandPath())
	}
	return nil
}
