package volumes

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/parse"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	volumePruneDescription = `By default, only anonymous (unnamed) unused volumes are removed.
  Use --all to remove all unused volumes (anonymous and named).
  The command prompts for confirmation which can be overridden with the --force flag.`
	pruneCommand = &cobra.Command{
		Use:               "prune [options]",
		Args:              validate.NoArgs,
		Short:             "Remove unused volumes",
		Long:              volumePruneDescription,
		RunE:              prune,
		ValidArgsFunction: completion.AutocompleteNone,
	}
	filter = []string{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pruneCommand,
		Parent:  volumeCmd,
	})
	flags := pruneCommand.Flags()

	filterFlagName := "filter"
	flags.StringArrayVar(&filter, filterFlagName, []string{}, "Provide filter values (e.g. 'label=<key>=<value>')")
	_ = pruneCommand.RegisterFlagCompletionFunc(filterFlagName, common.AutocompleteVolumePruneFilters)
	flags.BoolP("force", "f", false, "Do not prompt for confirmation")
	flags.BoolP("all", "a", false, "Remove all unused volumes, both anonymous and named")
	flags.Bool("dry-run", false, "Show what would be pruned without actually pruning")
}

func prune(cmd *cobra.Command, _ []string) error {
	var (
		pruneOptions  = entities.VolumePruneOptions{}
		listOptions   = entities.VolumeListOptions{}
		unusedOptions = entities.VolumeListOptions{}
	)
	// Prompt for confirmation if --force is not set
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	pruneOptions.Filters, err = parse.FilterArgumentsIntoFilters(filter)
	if err != nil {
		return err
	}

	// --all adds filter all=true (Docker-compatible; behavior is filter-only)
	allFlag, _ := cmd.Flags().GetBool("all")
	filterAllFlag := strings.EqualFold(pruneOptions.Filters.Get("all"), "true")
	if allFlag && filterAllFlag {
		return errors.New("--all and --filter all cannot be used together")
	}
	allFlag = allFlag || filterAllFlag
	if allFlag {
		pruneOptions.Filters.Set("all", "true")
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if force && dryRun {
		return errors.New("--force and --dry-run cannot be used together")
	}
	pruneOptions.DryRun = dryRun

	if !force && !dryRun {
		reader := bufio.NewReader(os.Stdin)
		if allFlag {
			fmt.Println("WARNING! This will remove all volumes not used by at least one container. The following volumes will be removed:")
		} else {
			fmt.Println("WARNING! This will remove anonymous local volumes not used by at least one container. The following volumes will be removed:")
		}
		listOptions.Filter, err = parse.FilterArgumentsIntoFilters(filter)
		if err != nil {
			return err
		}
		delete(listOptions.Filter, "all") // list does not support --filter all
		filteredVolumes, err := registry.ContainerEngine().VolumeList(context.Background(), listOptions)
		if err != nil {
			return err
		}
		var finalVolumes []*entities.VolumeListReport
		if allFlag {
			unusedOptions.Filter = map[string][]string{"dangling": {"true"}}
			unusedVolumes, err := registry.ContainerEngine().VolumeList(context.Background(), unusedOptions)
			if err != nil {
				return err
			}
			finalVolumes = getIntersection(unusedVolumes, filteredVolumes)
		} else {
			danglingOptions := entities.VolumeListOptions{Filter: map[string][]string{"dangling": {"true"}}}
			anonymousOptions := entities.VolumeListOptions{Filter: map[string][]string{"anonymous": {"true"}}}
			danglingVolumes, err := registry.ContainerEngine().VolumeList(context.Background(), danglingOptions)
			if err != nil {
				return err
			}
			anonymousVolumes, err := registry.ContainerEngine().VolumeList(context.Background(), anonymousOptions)
			if err != nil {
				return err
			}
			finalVolumes = getIntersection(getIntersection(danglingVolumes, anonymousVolumes), filteredVolumes)
		}
		if len(finalVolumes) < 1 {
			if allFlag {
				fmt.Println("No dangling volumes found")
			} else {
				fmt.Println("No dangling anonymous volumes found")
			}
			return nil
		}
		for _, fv := range finalVolumes {
			fmt.Println(fv.Name)
		}
		fmt.Print("Are you sure you want to continue? [y/N] ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.ToLower(answer)[0] != 'y' {
			return nil
		}
	}
	responses, err := registry.ContainerEngine().VolumePrune(context.Background(), pruneOptions)
	if err != nil {
		return err
	}
	if dryRun {
		fmt.Println("Volumes that would be pruned:")
	}
	return utils.PrintVolumePruneResults(responses, false)
}

func getIntersection(a, b []*entities.VolumeListReport) []*entities.VolumeListReport {
	var intersection []*entities.VolumeListReport
	hash := make(map[string]bool, len(a))
	for _, aa := range a {
		hash[aa.Name] = true
	}
	for _, bb := range b {
		if hash[bb.Name] {
			intersection = append(intersection, bb)
		}
	}
	return intersection
}
