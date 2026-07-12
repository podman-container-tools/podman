package farm

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/farm"
)

type buildOptions struct {
	buildOptions common.BuildFlagsWrapper
	local        bool
	platforms    []string
	farm         string
}

var (
	farmBuildDescription = `Build images on farm nodes, then bundle them into a manifest list`
	buildCommand         = &cobra.Command{
		Use:               "build [options] [CONTEXT]",
		Short:             "Build a container image for multiple architectures",
		Long:              farmBuildDescription,
		RunE:              build,
		Example:           "podman farm build [flags] buildContextDirectory",
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Args:              cobra.MaximumNArgs(1),
	}
	buildOpts = buildOptions{
		buildOptions: common.BuildFlagsWrapper{},
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildCommand,
		Parent:  farmCmd,
	})
	flags := buildCommand.Flags()
	flags.SetNormalizeFunc(utils.AliasFlags)

	cleanupFlag := "cleanup"
	flags.BoolVar(&buildOpts.buildOptions.Cleanup, cleanupFlag, false, "Remove built images from farm nodes on success")

	farmFlagName := "farm"
	flags.StringVar(&buildOpts.farm, farmFlagName, "", "Farm to use for builds")
	_ = buildCommand.RegisterFlagCompletionFunc(farmFlagName, common.AutoCompleteFarms)

	localFlagName := "local"
	// Default for local is true
	flags.BoolVarP(&buildOpts.local, localFlagName, "l", true, "Build image on local machine as well as on farm nodes")

	platformsFlag := "platforms"
	buildCommand.PersistentFlags().StringSliceVar(&buildOpts.platforms, platformsFlag, nil, "Build only on farm nodes that match the given platforms")
	_ = buildCommand.RegisterFlagCompletionFunc(platformsFlag, completion.AutocompletePlatform)

	common.DefineBuildFlags(buildCommand, &buildOpts.buildOptions, true)
}

func build(cmd *cobra.Command, args []string) error {
	// Return error if any of the hidden flags are used
	for _, f := range common.FarmBuildHiddenFlags {
		if cmd.Flags().Changed(f) {
			return fmt.Errorf("%q is an unsupported flag for podman farm build", f)
		}
	}

	if !cmd.Flags().Changed("tag") {
		return errors.New("cannot create manifest list without a name, value for --tag is required")
	}
	// Ensure that the user gives a full name so we can push the built images from
	// the node to the given registry and repository
	// Should be of the format registry/repository/imageName
	tag, err := cmd.Flags().GetStringArray("tag")
	if err != nil {
		return err
	}
	if !strings.Contains(tag[0], "/") {
		return fmt.Errorf("%q is not a full image reference name", tag[0])
	}
	bopts := buildOpts.buildOptions
	opts, err := common.ParseBuildOpts(cmd, args, &bopts)
	if err != nil {
		return err
	}
	// Close the logFile if one was created based on the flag
	if opts.LogFileToClose != nil {
		defer opts.LogFileToClose.Close()
	}
	if opts.TmpDirToClose != "" {
		// We had to download the context directory.
		// Delete it later.
		defer func() {
			if err = os.RemoveAll(opts.TmpDirToClose); err != nil {
				logrus.Errorf("Removing temporary directory %q: %v", opts.TmpDirToClose, err)
			}
		}()
	}
	opts.Cleanup = buildOpts.buildOptions.Cleanup
	iidFile, err := cmd.Flags().GetString("iidfile")
	if err != nil {
		return err
	}
	opts.IIDFile = iidFile
	iidFileRaw, err := cmd.Flags().GetString("iidfile-raw")
	if err != nil {
		return err
	}
	opts.IIDFileRaw = iidFileRaw
	// only set tls-verify if it has been changed by the user
	// if it hasn't we will read the registries.conf on the farm
	// nodes for further configuration
	if changed := cmd.Flags().Changed("tls-verify"); changed {
		tlsVerify, err := cmd.Flags().GetBool("tls-verify")
		if err != nil {
			return err
		}
		skipTLSVerify := !tlsVerify
		opts.SkipTLSVerify = &skipTLSVerify
	}

	localEngine := registry.ImageEngine()
	ctx := registry.Context()
	farm, err := farm.NewFarm(ctx, buildOpts.farm, localEngine, buildOpts.local)
	if err != nil {
		return fmt.Errorf("initializing: %w", err)
	}

	schedule, err := farm.Schedule(ctx, buildOpts.platforms)
	if err != nil {
		return fmt.Errorf("scheduling builds: %w", err)
	}
	logrus.Infof("schedule: %v", schedule)

	manifestName := opts.Output
	// Set Output to "" so that the images built on the farm nodes have no name
	opts.Output = ""
	if err = farm.Build(ctx, schedule, *opts, manifestName, localEngine); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	logrus.Infof("build: ok")

	return nil
}
