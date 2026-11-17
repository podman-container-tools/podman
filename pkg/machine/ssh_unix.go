//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package machine

import (
	"io"
	"os"
	"os/exec"
)

func setupIOPassthrough(cmd *exec.Cmd, _ bool, stdin io.Reader) error {
	cmd.Stdin = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return nil
}
