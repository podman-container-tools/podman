//go:build windows && (amd64 || arm64)

package machine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	psutil "github.com/shirou/gopsutil/v4/process"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/fileserver"
)

var server9pCommand = &cobra.Command{
	Args:              cobra.ExactArgs(1),
	Use:               "server9p [options] PID",
	Hidden:            true,
	Short:             "Serve a directory using 9p over hvsock",
	Long:              "Start a number of 9p servers on given hvsock UUIDs, and run until the given PID exits",
	RunE:              remoteDirServer,
	ValidArgsFunction: completion.AutocompleteNone,
	Example:           `podman system server9p --serve C:\Users\myuser:00000050-FACB-11E6-BD58-64006A7986D3 /mnt`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: server9pCommand,
		Parent:  machineCmd,
	})

	flags := server9pCommand.Flags()

	serveFlagName := "serve"
	flags.StringArrayVar(&serveDirs, serveFlagName, []string{}, "directories to serve and UUID of vsock to serve on, colon-separated")
	_ = server9pCommand.RegisterFlagCompletionFunc(serveFlagName, completion.AutocompleteNone)
}

var serveDirs []string

func remoteDirServer(_ *cobra.Command, args []string) error {
	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("parsing PID: %w", err)
	}
	if pid < 0 {
		return fmt.Errorf("PIDs cannot be negative")
	}

	if len(serveDirs) == 0 {
		return fmt.Errorf("must provide at least one directory to serve")
	}

	// TODO: need to support options here
	shares := make(map[string]string, len(serveDirs))
	for _, share := range serveDirs {
		splitShare := strings.Split(share, ":")
		if len(splitShare) < 2 {
			return fmt.Errorf("paths passed to --share must include an hvsock GUID")
		}

		// Every element but the last one is the real filepath to share
		path := strings.Join(splitShare[:len(splitShare)-1], ":")

		shares[path] = splitShare[len(splitShare)-1]
	}

	if err := fileserver.StartShares(shares); err != nil {
		return err
	}

	p, err := psutil.NewProcess(int32(pid))
	if err != nil {
		return fmt.Errorf("opening gvproxy PID: %w", err)
	}
	for {
		running, err := p.IsRunning()
		if err != nil {
			return fmt.Errorf("checking is gvproxy is dead: %w", err)
		}
		if !running {
			break
		}

		time.Sleep(1 * time.Second)
	}

	logrus.Infof("Exiting cleanly as PID %d has died", pid)

	return nil
}
