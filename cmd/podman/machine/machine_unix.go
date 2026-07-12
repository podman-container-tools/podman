//go:build linux || aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || netbsd || openbsd || solaris

package machine

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/pkg/rootless"
)

func isUnixSocket(file os.DirEntry) bool {
	return file.Type()&os.ModeSocket != 0
}

func rootlessOnly(cmd *cobra.Command, _ []string) error {
	if !rootless.IsRootless() {
		return fmt.Errorf("cannot run command %q as root", cmd.CommandPath())
	}
	return nil
}
