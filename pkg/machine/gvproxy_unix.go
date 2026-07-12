//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package machine

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	psutil "github.com/shirou/gopsutil/v4/process"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"golang.org/x/sys/unix"
)

const (
	loops     = 8
	sleepTime = time.Millisecond * 1
)

// backoffForProcess checks if the process still exists, for something like
// sigterm. If the process still exists after loops and sleep time are exhausted,
// an error is returned
func backoffForProcess(p *psutil.Process) error {
	sleepInterval := sleepTime
	for range loops {
		running, err := p.IsRunning()
		if err != nil {
			// It is possible that while in our loop, the PID vaporize triggering
			// an input/output error (#21845)
			if errors.Is(err, unix.EIO) {
				return nil
			}
			return fmt.Errorf("checking if process running: %w", err)
		}
		if !running {
			return nil
		}

		time.Sleep(sleepInterval)
		// double the time
		sleepInterval += sleepInterval
	}
	return fmt.Errorf("process has not ended (PID %d)", p.Pid)
}

// waitOnProcess takes a pid and sends a sigterm to it. it then waits for the
// process to not exist.  if the sigterm does not end the process after an interval,
// then sigkill is sent.  it also waits for the process to exit after the sigkill too.
func waitOnProcess(processID int) error {
	logrus.Infof("Going to stop gvproxy (PID %d)", processID)

	p, err := psutil.NewProcess(int32(processID))
	if err != nil {
		return fmt.Errorf("looking up PID %d: %w", processID, err)
	}

	running, err := p.IsRunning()
	if err != nil {
		return fmt.Errorf("checking if gvproxy is running: %w", err)
	}
	if !running {
		return nil
	}

	// Start a goroutine that waits until the gvproxy process completes.
	// This is necessary to reaps the process and so that Process.IsRunning()
	// in backoffForProcess() returns false. Otherwise the process will
	// be defunct and backoffForProcess fails because Process.IsRunning()
	// returns true
	go func() {
		gv, err := os.FindProcess(processID)
		if err != nil {
			logrus.Errorf("failed to find process %d: %v", processID, err)
			return
		}
		if _, err = gv.Wait(); err != nil {
			logrus.Debugf("gvproxy exited: %v", err)
		}
	}()

	if err = p.Terminate(); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			logrus.Debugf("Gvproxy already dead, exiting cleanly")
			return nil
		}
		return err
	}

	if err = backoffForProcess(p); err == nil {
		return nil
	}

	if err = p.Kill(); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			logrus.Debugf("Gvproxy already dead, exiting cleanly")
			return nil
		}
		return err
	}
	return backoffForProcess(p)
}

// removeGVProxyPIDFile is just a wrapper to vmfile delete so we handle differently
// on windows
func removeGVProxyPIDFile(f define.VMFile) error {
	return f.Delete()
}
