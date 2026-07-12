//go:build amd64 || arm64

package os

import (
	"context"
	"errors"

	"github.com/blang/semver/v4"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/machine"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/machine/os"
	"go.podman.io/podman/v6/version"
)

var upgradeCmd = &cobra.Command{
	Use:               "upgrade [options] [NAME]",
	Short:             "Upgrade machine os",
	Long:              "Upgrade the machine operating system to a newer version",
	PersistentPreRunE: validate.NoOp,
	Args:              cobra.MaximumNArgs(1),
	RunE:              upgrade,
	ValidArgsFunction: common.AutocompleteImages,
	Example:           `podman machine os upgrade`,
}

type upgradeOpts struct {
	dryRun      bool
	format      string
	hostVersion string
	restart     bool
}

var opts upgradeOpts

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: upgradeCmd,
		Parent:  machine.OSCmd,
	})
	flags := upgradeCmd.Flags()

	dryRunFlagName := "dry-run"
	flags.BoolVarP(&opts.dryRun, dryRunFlagName, "n", false, "Only check if an upgrade is available")
	hostVersionFlagName := "host-version"
	flags.StringVar(&opts.hostVersion, hostVersionFlagName, "", "Podman host version (advanced use only)")
	_ = upgradeCmd.RegisterFlagCompletionFunc(hostVersionFlagName, completion.AutocompleteNone)
	_ = flags.MarkHidden(hostVersionFlagName)
	formatFlagName := "format"
	flags.StringVarP(&opts.format, formatFlagName, "f", "", "suppress output except for specified format. Implies -n")
	_ = upgradeCmd.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteNone)
	restartFlagName := "restart"
	flags.BoolVar(&opts.restart, restartFlagName, false, "Restart VM to upgrade")
}

func upgrade(_ *cobra.Command, args []string) error {
	var vmName string
	if len(args) == 1 {
		vmName = args[0]
	}

	managerOpts := ManagerOpts{
		VMName:  vmName,
		CLIArgs: args,
		Restart: opts.restart,
	}

	osManager, err := NewOSManager(managerOpts)
	if err != nil {
		return err
	}

	upgradeOpts := os.UpgradeOptions{ClientVersion: version.Version, DryRun: opts.dryRun}
	if opts.hostVersion != "" {
		callerVersion, err := semver.ParseTolerant(opts.hostVersion)
		if err != nil {
			return err
		}
		upgradeOpts.MachineVersion = callerVersion
	}
	if kind := opts.format; len(opts.format) > 0 {
		// For now, we only support one output format value of `json`
		// But in the future, we may add additional formats as needed
		if kind != "json" {
			return errors.New("only value of `json` is supported")
		}
		upgradeOpts.Format = kind
	}
	if opts.restart && (len(upgradeOpts.Format) > 0 || opts.dryRun) {
		return errors.New("--restart cannot be used with --dry-run or --format")
	}

	return osManager.Upgrade(context.Background(), upgradeOpts)
}
