package images

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// setupPipe for fixing https://github.com/containers/podman/issues/7017
// uses named pipe since containers/image EvalSymlinks fails with /dev/stdout
// the caller should use the returned function to clean up the pipeDir
func setupPipe() (string, func() <-chan error, error) {
	errc := make(chan error)
	pipeDir, err := os.MkdirTemp(os.TempDir(), "pipeDir")
	if err != nil {
		return "", nil, err
	}
	pipePath := filepath.Join(pipeDir, "saveio")
	err = unix.Mkfifo(pipePath, 0o600)
	if err != nil {
		if e := os.RemoveAll(pipeDir); e != nil {
			logrus.Errorf("Removing named pipe: %q", e)
		}
		return "", nil, fmt.Errorf("creating named pipe: %w", err)
	}
	go func() {
		fpipe, err := os.Open(pipePath)
		if err != nil {
			errc <- err
			return
		}
		_, err = io.Copy(os.Stdout, fpipe)
		fpipe.Close()
		errc <- err
	}()
	return pipePath, func() <-chan error {
		if e := os.RemoveAll(pipeDir); e != nil {
			logrus.Errorf("Removing named pipe: %q", e)
		}
		return errc
	}, nil
}
