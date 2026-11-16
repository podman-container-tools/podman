//go:build !linux

package passdriver

import (
	"context"
	"os/exec"
)

func gpgCommand(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, program, args...), nil
}
