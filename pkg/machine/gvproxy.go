package machine

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"time"

	"go.podman.io/podman/v6/pkg/machine/define"
)

const (
	pidFileWaitTimeout   = 2 * time.Second
	pidFileCheckInterval = 50 * time.Millisecond
)

func readPIDFileWithRetry(f define.VMFile) ([]byte, error) {
	deadline := time.Now().Add(pidFileWaitTimeout)

	for time.Now().Before(deadline) {
		gvPid, err := f.Read()
		if err == nil {
			return gvPid, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		time.Sleep(pidFileCheckInterval)
	}

	// Final attempt after timeout
	return f.Read()
}

// CleanupGVProxy reads the --pid-file for gvproxy attempts to stop it
func CleanupGVProxy(f define.VMFile) error {
	gvPid, err := readPIDFileWithRetry(f)
	if err != nil {
		// The file will also be removed by gvproxy when it exits so
		// we need to account for the race and can just ignore it here.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("unable to read gvproxy pid file: %v", err)
	}
	proxyPid, err := strconv.Atoi(string(gvPid))
	if err != nil {
		return fmt.Errorf("unable to convert pid to integer: %v", err)
	}
	if err := waitOnProcess(proxyPid); err != nil {
		return err
	}
	return removeGVProxyPIDFile(f)
}
