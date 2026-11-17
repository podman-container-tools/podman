package machine

import (
	"errors"
	"os"
	"time"

	"github.com/containers/winquit/pkg/winquit"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"golang.org/x/sys/windows"
)

func waitOnProcess(processID int) error {
	logrus.Infof("Going to stop gvproxy (PID %d)", processID)

	p, err := os.FindProcess(processID)
	if err != nil {
		// FindProcess on Windows will return an error when the process is not found
		// if a process can not be found then it has already exited and there is
		// nothing left to do, so return without error
		//nolint:nilerr
		return nil
	}

	// Gracefully quit and force kill after 30 seconds
	if err := winquit.QuitProcess(processID, 30*time.Second); err != nil {
		return err
	}

	logrus.Debugf("completed grace quit || kill of gvproxy (PID %d)", processID)

	// Make sure the process is gone (Hard kills are async)
	done := make(chan struct{})
	go func() {
		_, _ = p.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		logrus.Debugf("verified gvproxy termination (PID %d)", processID)
	case <-time.After(10 * time.Second):
		// Very unlikely but track just in case
		logrus.Errorf("was not able to kill gvproxy (PID %d)", processID)
	}

	return nil
}

// removeGVProxyPIDFile special wrapper for deleting the GVProxyPIDFile on windows in case
// the file has an open handle which we will ignore.  unix does not have this problem
func removeGVProxyPIDFile(f define.VMFile) error {
	err := f.Delete()
	if err != nil && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return err
	}
	return nil
}
