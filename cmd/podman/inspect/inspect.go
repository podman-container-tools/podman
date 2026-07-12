package inspect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/report"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// AddInspectFlagSet takes a command and adds the inspect flags and returns an
// InspectOptions object.
func AddInspectFlagSet(cmd *cobra.Command) *entities.InspectOptions {
	opts := entities.InspectOptions{}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.Size, "size", "s", false, "Display total file size")

	formatFlagName := "format"
	flags.StringVarP(&opts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = cmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(nil)) // passing nil as the type selection logic is in AutocompleteFormat function

	typeFlagName := "type"
	flags.StringVarP(&opts.Type, typeFlagName, "t", common.AllType, "Specify inspect-object type")
	_ = cmd.RegisterFlagCompletionFunc(typeFlagName, common.AutocompleteInspectType)

	validate.AddLatestFlag(cmd, &opts.Latest)
	return &opts
}

// Inspect inspects the specified container/image/pod/volume names or IDs.
func Inspect(namesOrIDs []string, options entities.InspectOptions) error {
	inspector, err := newInspector(options)
	if err != nil {
		return err
	}
	return inspector.inspect(namesOrIDs)
}

// inspector allows for inspecting images and containers.
type inspector struct {
	containerEngine entities.ContainerEngine
	imageEngine     entities.ImageEngine
	options         entities.InspectOptions
}

// newInspector creates a new inspector based on the specified options.
func newInspector(options entities.InspectOptions) (*inspector, error) {
	if options.Type == common.ImageType {
		if options.Latest {
			return nil, fmt.Errorf("latest is not supported for type %q", common.ImageType)
		}
		if options.Size {
			return nil, fmt.Errorf("size is not supported for type %q", common.ImageType)
		}
	}
	if options.Type == common.PodType && options.Size {
		return nil, fmt.Errorf("size is not supported for type %q", common.PodType)
	}
	return &inspector{
		containerEngine: registry.ContainerEngine(),
		imageEngine:     registry.ImageEngine(),
		options:         options,
	}, nil
}

// inspect inspects the specified container/image names or IDs.
func (i *inspector) inspect(namesOrIDs []string) error {
	// data - dumping place for inspection results.
	var data []any
	var errs []error
	ctx := context.Background()

	if len(namesOrIDs) == 0 {
		if !i.options.Latest && !i.options.All {
			return errors.New("no names or ids specified")
		}
	}

	tmpType := i.options.Type
	if i.options.Latest {
		if len(namesOrIDs) > 0 {
			return errors.New("--latest and arguments cannot be used together")
		}
		if i.options.Type == common.AllType {
			tmpType = common.ContainerType // -l works with --type=all, defaults to containertype
		}
	}

	// Inspect - note that AllType requires us to expensively query one-by-one.
	switch tmpType {
	case common.AllType:
		allData, allErrs, err := i.inspectAll(ctx, namesOrIDs)
		if err != nil {
			return err
		}
		data = allData
		errs = allErrs
	case common.ImageType:
		imgData, allErrs, err := i.imageEngine.Inspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range imgData {
			data = append(data, imgData[i])
		}
	case common.ContainerType:
		ctrData, allErrs, err := i.containerEngine.ContainerInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range ctrData {
			data = append(data, ctrData[i])
		}
	case common.PodType:
		podData, allErrs, err := i.containerEngine.PodInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range podData {
			data = append(data, podData[i])
		}

	case common.NetworkType:
		networkData, allErrs, err := registry.ContainerEngine().NetworkInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range networkData {
			data = append(data, networkData[i])
		}
	case common.VolumeType:
		volumeData, allErrs, err := i.containerEngine.VolumeInspect(ctx, namesOrIDs, i.options)
		if err != nil {
			return err
		}
		errs = allErrs
		for i := range volumeData {
			data = append(data, volumeData[i])
		}
	case common.ArtifactType:
		for _, name := range namesOrIDs {
			artifactData, err := i.imageEngine.ArtifactInspect(ctx, name, entities.ArtifactInspectOptions{})
			if err != nil {
				errs = append(errs, err)
				continue
			}
			data = append(data, artifactData)
		}
	default:
		return fmt.Errorf("invalid type %q: must be %q, %q, %q, %q, %q, %q, or %q", i.options.Type,
			common.ImageType, common.ContainerType, common.PodType, common.NetworkType, common.VolumeType, common.ArtifactType, common.AllType)
	}
	// Always print an empty array
	if data == nil {
		data = []any{}
	}

	var err error
	switch {
	case report.IsJSON(i.options.Format) || i.options.Format == "":
		err = utils.PrintGenericJSON(data)
	default:
		// Landing here implies user has given a custom --format
		var rpt *report.Formatter
		format := InspectNormalize(i.options.Format, i.options.Type)
		rpt, err = report.New(os.Stdout, "inspect").Parse(report.OriginUser, format)
		if err != nil {
			return err
		}
		defer rpt.Flush()

		err = rpt.Execute(data)
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

func (i *inspector) inspectAll(ctx context.Context, namesOrIDs []string) ([]any, []error, error) {
	var data []any
	allErrs := []error{}
	for _, name := range namesOrIDs {
		ctrData, errs, err := i.containerEngine.ContainerInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, ctrData[0])
			continue
		}
		imgData, errs, err := i.imageEngine.Inspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, imgData[0])
			continue
		}
		volumeData, errs, err := i.containerEngine.VolumeInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, volumeData[0])
			continue
		}
		networkData, errs, err := registry.ContainerEngine().NetworkInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, networkData[0])
			continue
		}

		podData, errs, err := i.containerEngine.PodInspect(ctx, []string{name}, i.options)
		if err != nil {
			return nil, nil, err
		}
		if len(errs) == 0 {
			data = append(data, podData[0])
			continue
		}
		artifactData, err := i.imageEngine.ArtifactInspect(ctx, name, entities.ArtifactInspectOptions{})
		if err == nil {
			data = append(data, artifactData)
			continue
		}
		if len(errs) > 0 {
			allErrs = append(allErrs, fmt.Errorf("no such object: %q", name))
			continue
		}
	}
	return data, allErrs, nil
}

// InspectNormalize modifies a given row string based on the specified inspect type.
// It replaces specific field names within the row string for standardization.
// For the `image` inspect type, it includes additional field replacements like `.Config.Healthcheck`.
//
// Parameters:
// - row: The input string that represents a data row to be modified.
// - inspectType: The type of inspection (e.g., "image") to determine specific replacements.
//
// Returns:
// - A new string with the necessary replacements applied based on the inspect type.
//
// InspectNormalize does not need to be exported but to avoid de-duplication of code. We had to export it.
// It can be reverted back once `podman artifact inspect` can use [Inspect] to fetch artifact data instead of
// fetching it itself.
// The reason why we did it in this way can be further read [here](https://github.com/containers/podman/pull/27182#issuecomment-3402465389).
func InspectNormalize(row string, inspectType string) string {
	m := regexp.MustCompile(`{{\s*\.Id\s*}}`)
	row = m.ReplaceAllString(row, "{{.ID}}")

	r := strings.NewReplacer(
		".Src", ".Source",
		".Dst", ".Destination",
		".ImageID", ".Image",
	)

	// If inspect type is `image` we need to replace
	// certain additional fields like `.Config.HealthCheck`
	// but don't want to replace them for other inspect types.
	if inspectType == common.ImageType {
		r = strings.NewReplacer(
			".Src", ".Source",
			".Dst", ".Destination",
			".ImageID", ".Image",
			".Config.Healthcheck", ".HealthCheck",
		)
	}

	return r.Replace(row)
}
