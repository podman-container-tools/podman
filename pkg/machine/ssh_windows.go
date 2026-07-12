package machine

import (
	"io"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

func setupIOPassthrough(cmd *exec.Cmd, interactive bool, stdin io.Reader) error {
	cmd.Stdin = stdin

	if interactive {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return nil
	}

	// OpenSSh mucks with the associated virtual console when there is no pty,
	// leaving it in a broken state. Pipe the output to isolate stdout/stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	copier := func(name string, dest string, from io.Reader, to io.Writer) {
		if _, err := io.Copy(to, from); err != nil {
			logrus.Warnf("could not copy output from command %s to %s", name, dest)
		}
	}

	go copier(cmd.Path, "stdout", stdout, os.Stdout)
	go copier(cmd.Path, "stderr", stderr, os.Stderr)

	return nil
}
