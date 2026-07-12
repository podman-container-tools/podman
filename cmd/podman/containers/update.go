package containers

import (
	"context"
	"fmt"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/util"
)

var (
	updateDescription = `Updates the configuration of an existing container, allowing changes to resource limits and healthchecks`

	updateCommand = &cobra.Command{
		Use:               "update [options] CONTAINER",
		Short:             "Update an existing container",
		Long:              updateDescription,
		RunE:              update,
		Args:              validate.IDOrLatestArgs,
		ValidArgsFunction: common.AutocompleteContainers,
		Example:           `podman update --cpus=5 foobar_container`,
	}

	containerUpdateCommand = &cobra.Command{
		Args:              updateCommand.Args,
		Use:               updateCommand.Use,
		Short:             updateCommand.Short,
		Long:              updateCommand.Long,
		RunE:              updateCommand.RunE,
		ValidArgsFunction: updateCommand.ValidArgsFunction,
		Example:           `podman container update --cpus=5 foobar_container`,
	}
)

type ContainerUpdateOptions struct {
	entities.ContainerCreateOptions
	Latest bool
}

var updateOptions ContainerUpdateOptions

func updateFlags(cmd *cobra.Command) {
	common.DefineCreateDefaults(&updateOptions.ContainerCreateOptions)
	common.DefineCreateFlags(cmd, &updateOptions.ContainerCreateOptions, entities.UpdateMode)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: updateCommand,
	})
	updateFlags(updateCommand)
	validate.AddLatestFlag(updateCommand, &updateOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUpdateCommand,
		Parent:  containerCmd,
	})
	updateFlags(containerUpdateCommand)
	validate.AddLatestFlag(containerUpdateCommand, &updateOptions.Latest)
}

func GetChangedHealthCheckConfiguration(cmd *cobra.Command, vals *entities.ContainerCreateOptions) define.UpdateHealthCheckConfig {
	updateHealthCheckConfig := define.UpdateHealthCheckConfig{}

	if cmd.Flags().Changed("health-log-destination") {
		updateHealthCheckConfig.HealthLogDestination = &vals.HealthLogDestination
	}
	if cmd.Flags().Changed("health-max-log-size") {
		updateHealthCheckConfig.HealthMaxLogSize = &vals.HealthMaxLogSize
	}
	if cmd.Flags().Changed("health-max-log-count") {
		updateHealthCheckConfig.HealthMaxLogCount = &vals.HealthMaxLogCount
	}
	if cmd.Flags().Changed("health-on-failure") {
		updateHealthCheckConfig.HealthOnFailure = &vals.HealthOnFailure
	}
	if cmd.Flags().Changed("no-healthcheck") {
		updateHealthCheckConfig.NoHealthCheck = &vals.NoHealthCheck
	}
	if cmd.Flags().Changed("health-cmd") {
		updateHealthCheckConfig.HealthCmd = &vals.HealthCmd
	}
	if cmd.Flags().Changed("health-interval") {
		updateHealthCheckConfig.HealthInterval = &vals.HealthInterval
	}
	if cmd.Flags().Changed("health-retries") {
		updateHealthCheckConfig.HealthRetries = &vals.HealthRetries
	}
	if cmd.Flags().Changed("health-timeout") {
		updateHealthCheckConfig.HealthTimeout = &vals.HealthTimeout
	}
	if cmd.Flags().Changed("health-start-period") {
		updateHealthCheckConfig.HealthStartPeriod = &vals.HealthStartPeriod
	}
	if cmd.Flags().Changed("health-startup-cmd") {
		updateHealthCheckConfig.HealthStartupCmd = &vals.StartupHCCmd
	}
	if cmd.Flags().Changed("health-startup-interval") {
		updateHealthCheckConfig.HealthStartupInterval = &vals.StartupHCInterval
	}
	if cmd.Flags().Changed("health-startup-retries") {
		updateHealthCheckConfig.HealthStartupRetries = &vals.StartupHCRetries
	}
	if cmd.Flags().Changed("health-startup-timeout") {
		updateHealthCheckConfig.HealthStartupTimeout = &vals.StartupHCTimeout
	}
	if cmd.Flags().Changed("health-startup-success") {
		updateHealthCheckConfig.HealthStartupSuccess = &vals.StartupHCSuccesses
	}

	return updateHealthCheckConfig
}

func GetChangedDeviceLimits(s *specgen.SpecGenerator) *define.UpdateContainerDevicesLimits {
	updateDevicesLimits := define.UpdateContainerDevicesLimits{}
	updateDevicesLimits.SetBlkIOWeightDevice(s.WeightDevice)
	updateDevicesLimits.SetDeviceReadBPs(s.ThrottleReadBpsDevice)
	updateDevicesLimits.SetDeviceWriteBPs(s.ThrottleWriteBpsDevice)
	updateDevicesLimits.SetDeviceReadIOPs(s.ThrottleReadIOPSDevice)
	updateDevicesLimits.SetDeviceWriteIOPs(s.ThrottleWriteIOPSDevice)
	return &updateDevicesLimits
}

func update(cmd *cobra.Command, args []string) error {
	var err error
	// use a specgen since this is the easiest way to hold resource info
	s := &specgen.SpecGenerator{}
	s.ResourceLimits = &specs.LinuxResources{}

	err = createOrUpdateFlags(cmd, &updateOptions.ContainerCreateOptions)
	if err != nil {
		return err
	}

	s.ResourceLimits, err = specgenutil.GetResources(s, &updateOptions.ContainerCreateOptions)
	if err != nil {
		return err
	}

	if s.ResourceLimits == nil {
		s.ResourceLimits = &specs.LinuxResources{}
	}

	healthCheckConfig := GetChangedHealthCheckConfiguration(cmd, &updateOptions.ContainerCreateOptions)

	opts := &entities.ContainerUpdateOptions{
		Resources:                       s.ResourceLimits,
		ChangedHealthCheckConfiguration: &healthCheckConfig,
		DevicesLimits:                   GetChangedDeviceLimits(s),
		Latest:                          updateOptions.Latest,
	}

	if !updateOptions.Latest {
		opts.NameOrID = strings.TrimPrefix(args[0], "/")
	}

	if cmd.Flags().Changed("restart") {
		policy, retries, err := util.ParseRestartPolicy(updateOptions.Restart)
		if err != nil {
			return err
		}
		opts.RestartPolicy = &policy
		if policy == define.RestartPolicyOnFailure {
			opts.RestartRetries = &retries
		}
	}

	if cmd.Flags().Changed("env") {
		env, err := cmd.Flags().GetStringArray("env")
		if err != nil {
			return err
		}
		opts.Env = env
	}

	if cmd.Flags().Changed("unsetenv") {
		env, err := cmd.Flags().GetStringArray("unsetenv")
		if err != nil {
			return err
		}
		opts.UnsetEnv = env
	}

	if cmd.Flags().Changed("ulimit") {
		ulimits, err := cmd.Flags().GetStringSlice("ulimit")
		if err != nil {
			return err
		}
		rlimits, err := specgenutil.GenRlimits(ulimits)
		if err != nil {
			return err
		}
		opts.Rlimits = rlimits
	}

	rep, err := registry.ContainerEngine().ContainerUpdate(context.Background(), opts)
	if err != nil {
		return err
	}
	fmt.Println(rep)
	return nil
}
