package system

import (
	"context"
	"errors"
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

var (
	checkOptions     = types.SystemCheckOptions{}
	checkDescription = `
	podman system check

        Check storage for consistency and remove anything that looks damaged
`

	checkCommand = &cobra.Command{
		Use:               "check [options]",
		Short:             "Check storage consistency",
		Args:              validate.NoArgs,
		Long:              checkDescription,
		RunE:              check,
		ValidArgsFunction: completion.AutocompleteNone,
		Example:           `podman system check`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: checkCommand,
		Parent:  systemCmd,
	})
	flags := checkCommand.Flags()
	flags.BoolVarP(&checkOptions.Quick, "quick", "q", false, "Skip time-consuming checks. The default is to include time-consuming checks")
	flags.BoolVarP(&checkOptions.Repair, "repair", "r", false, "Remove inconsistent images")
	flags.BoolVarP(&checkOptions.RepairLossy, "force", "f", false, "Remove inconsistent images and containers")
	flags.DurationP("max", "m", 24*time.Hour, "Maximum allowed age of unreferenced layers")
	_ = checkCommand.RegisterFlagCompletionFunc("max", completion.AutocompleteNone)
}

func check(cmd *cobra.Command, _ []string) error {
	flags := cmd.Flags()
	if flags.Changed("max") {
		maxAge, err := flags.GetDuration("max")
		if err != nil {
			return err
		}
		checkOptions.UnreferencedLayerMaximumAge = &maxAge
	}
	response, err := registry.ContainerEngine().SystemCheck(context.Background(), checkOptions)
	if err != nil {
		return err
	}

	if err = printSystemCheckResults(response); err != nil {
		return err
	}

	if !checkOptions.Repair && !checkOptions.RepairLossy {
		if response.Errors {
			return errors.New("damage detected in local storage")
		}
		return nil
	}

	recheckOptions := checkOptions
	recheckOptions.Repair = false
	recheckOptions.RepairLossy = false
	if response, err = registry.ContainerEngine().SystemCheck(context.Background(), recheckOptions); err != nil {
		return err
	}
	if response.Errors {
		return errors.New("damage in local storage still present after repair attempt")
	}

	return nil
}

func printSystemCheckResults(report *types.SystemCheckReport) error {
	if !report.Errors {
		return nil
	}
	errorSlice := func(strs []string) []error {
		if strs == nil {
			return nil
		}
		errs := make([]error, len(strs))
		for i, s := range strs {
			errs[i] = errors.New(s)
		}
		return errs
	}
	for damagedLayer, errorsSlice := range report.Layers {
		merr := multierror.Append(nil, errorSlice(errorsSlice)...)
		if err := merr.ErrorOrNil(); err != nil {
			fmt.Printf("Damaged layer %s:\n%s", damagedLayer, err)
		}
	}
	for _, removedLayer := range report.RemovedLayers {
		fmt.Printf("Deleted damaged layer: %s\n", removedLayer)
	}
	for damagedROLayer, errorsSlice := range report.ROLayers {
		merr := multierror.Append(nil, errorSlice(errorsSlice)...)
		if err := merr.ErrorOrNil(); err != nil {
			fmt.Printf("Damaged read-only layer %s:\n%s", damagedROLayer, err)
		}
	}
	for damagedImage, errorsSlice := range report.Images {
		merr := multierror.Append(nil, errorSlice(errorsSlice)...)
		if err := merr.ErrorOrNil(); err != nil {
			fmt.Printf("Damaged image %s:\n%s", damagedImage, err)
		}
	}
	for removedImage := range report.RemovedImages {
		fmt.Printf("Deleted damaged image: %s\n", removedImage)
	}
	for damagedROImage, errorsSlice := range report.ROImages {
		merr := multierror.Append(nil, errorSlice(errorsSlice)...)
		if err := merr.ErrorOrNil(); err != nil {
			fmt.Printf("Damaged read-only image %s\n%s", damagedROImage, err)
		}
	}
	for damagedContainer, errorsSlice := range report.Containers {
		merr := multierror.Append(nil, errorSlice(errorsSlice)...)
		if err := merr.ErrorOrNil(); err != nil {
			fmt.Printf("Damaged container %s:\n%s", damagedContainer, err)
		}
	}
	for removedContainer := range report.RemovedContainers {
		fmt.Printf("Deleted damaged container: %s\n", removedContainer)
	}
	return nil
}
