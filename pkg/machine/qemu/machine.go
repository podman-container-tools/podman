//go:build linux || freebsd || windows

package qemu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/storage/pkg/fileutils"
)

func NewStubber() (*QEMUStubber, error) {
	return &QEMUStubber{}, nil
}

// qemuPid returns -1 or the PID of the running QEMU instance.
func qemuPid(pidFile *define.VMFile) (int, error) {
	pidData, err := os.ReadFile(pidFile.GetPath())
	if err != nil {
		// The file may not yet exist on start or have already been
		// cleaned up after stop, so we need to be defensive.
		if errors.Is(err, os.ErrNotExist) {
			return -1, nil
		}
		return -1, err
	}
	if len(pidData) == 0 {
		return -1, nil
	}

	pid, err := strconv.Atoi(strings.TrimRight(string(pidData), "\n"))
	if err != nil {
		logrus.Warnf("Reading QEMU pidfile: %v", err)
		return -1, nil
	}
	return findProcess(pid)
}

// todo move this to qemumonitor stuff. it has no use as a method of stubber
func (q *QEMUStubber) checkStatus(monitor *qmp.SocketMonitor) (define.Status, error) {
	// this is the format returned from the monitor
	// {"return": {"status": "running", "singlestep": false, "running": true}}

	type statusDetails struct {
		Status   string `json:"status"`
		Step     bool   `json:"singlestep"`
		Running  bool   `json:"running"`
		Starting bool   `json:"starting"`
	}
	type statusResponse struct {
		Response statusDetails `json:"return"`
	}
	var response statusResponse

	checkCommand := struct {
		Execute string `json:"execute"`
	}{
		Execute: "query-status",
	}
	input, err := json.Marshal(checkCommand)
	if err != nil {
		return "", err
	}
	b, err := monitor.Run(input)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return define.Stopped, nil
		}
		return "", err
	}
	if err := json.Unmarshal(b, &response); err != nil {
		return "", err
	}
	if response.Response.Status == define.Running {
		return define.Running, nil
	}
	return define.Stopped, nil
}

// waitForMachineToStop waits for the machine to stop running
func (q *QEMUStubber) waitForMachineToStop(mc *vmconfigs.MachineConfig) error {
	fmt.Println("Waiting for VM to stop running...")
	waitInternal := 250 * time.Millisecond
	for range 5 {
		state, err := q.State(mc, false)
		if err != nil {
			return err
		}
		if state != define.Running {
			break
		}
		time.Sleep(waitInternal)
		waitInternal *= 2
	}
	// after the machine stops running it normally takes about 1 second for the
	// qemu VM to exit so we wait a bit to try to avoid issues
	time.Sleep(2 * time.Second)
	return nil
}

// Stop uses the qmp monitor to call a system_powerdown
func (q *QEMUStubber) StopVM(mc *vmconfigs.MachineConfig, _ bool) error {
	if err := mc.Refresh(); err != nil {
		return err
	}

	stopErr := q.stopLocked(mc)

	// Make sure that the associated QEMU process gets killed in case it's
	// still running (#16054).
	qemuPid, err := qemuPid(mc.QEMUHypervisor.QEMUPidPath)
	if err != nil {
		if stopErr == nil {
			return err
		}
		return fmt.Errorf("%w: %w", stopErr, err)
	}

	if qemuPid == -1 {
		return stopErr
	}

	if err := sigKill(qemuPid); err != nil {
		if stopErr == nil {
			return err
		}
		return fmt.Errorf("%w: %w", stopErr, err)
	}

	return stopErr
}

// stopLocked stops the machine and expects the caller to hold the machine's lock.
func (q *QEMUStubber) stopLocked(mc *vmconfigs.MachineConfig) error {
	// check if the qmp socket is there. if not, qemu instance is gone
	if err := fileutils.Exists(mc.QEMUHypervisor.QMPMonitor.Address.GetPath()); errors.Is(err, fs.ErrNotExist) {
		// Right now it is NOT an error to stop a stopped machine
		logrus.Debugf("QMP monitor socket %v does not exist", mc.QEMUHypervisor.QMPMonitor.Address)
		// Fix incorrect starting state in case of crash during start
		if mc.Starting {
			mc.Starting = false
			if err := mc.Write(); err != nil {
				return err
			}
		}
		return nil
	}

	qmpMonitor, err := qmp.NewSocketMonitor(mc.QEMUHypervisor.QMPMonitor.Network, mc.QEMUHypervisor.QMPMonitor.Address.GetPath(), mc.QEMUHypervisor.QMPMonitor.Timeout)
	if err != nil {
		return err
	}
	// Simple JSON formation for the QAPI
	stopCommand := struct {
		Execute string `json:"execute"`
	}{
		Execute: "system_powerdown",
	}

	input, err := json.Marshal(stopCommand)
	if err != nil {
		return err
	}

	if err := qmpMonitor.Connect(); err != nil {
		return err
	}

	var disconnected bool
	defer func() {
		if !disconnected {
			if err := qmpMonitor.Disconnect(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if _, err = qmpMonitor.Run(input); err != nil {
		return err
	}

	// Remove socket
	if err := mc.QEMUHypervisor.QMPMonitor.Address.Delete(); err != nil {
		return err
	}

	if err := qmpMonitor.Disconnect(); err != nil {
		// FIXME: this error should probably be returned
		return nil
	}
	disconnected = true

	if mc.QEMUHypervisor.QEMUPidPath.GetPath() == "" {
		// no vm pid file path means it's probably a machine created before we
		// started using it, so we revert to the old way of waiting for the
		// machine to stop
		return q.waitForMachineToStop(mc)
	}

	vmPid, err := mc.QEMUHypervisor.QEMUPidPath.ReadPIDFrom()
	if err != nil {
		return err
	}

	fmt.Println("Waiting for VM to exit...")
	for isProcessAlive(vmPid) {
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// Remove deletes all the files associated with a machine including the image itself
func (q *QEMUStubber) Remove(mc *vmconfigs.MachineConfig) ([]string, func() error, error) {
	qemuRmFiles := []string{
		mc.QEMUHypervisor.QEMUPidPath.GetPath(),
		mc.QEMUHypervisor.QMPMonitor.Address.GetPath(),
	}

	return qemuRmFiles, func() error {
		var errs []error
		if err := mc.QEMUHypervisor.QEMUPidPath.Delete(); err != nil {
			errs = append(errs, err)
		}

		if err := mc.QEMUHypervisor.QMPMonitor.Address.Delete(); err != nil {
			errs = append(errs, err)
		}
		return errorhandling.JoinErrors(errs)
	}, nil
}

func (q *QEMUStubber) State(mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	// Check if qmp socket path exists
	if err := fileutils.Exists(mc.QEMUHypervisor.QMPMonitor.Address.GetPath()); errors.Is(err, fs.ErrNotExist) {
		return define.Stopped, nil
	}
	if err := mc.Refresh(); err != nil {
		return "", err
	}

	// TODO this has always been a problem, lets fix this
	// Check if we can dial it
	// if v.Starting && !bypass {
	// 	return define.Starting, nil
	// }

	monitor, err := qmp.NewSocketMonitor(mc.QEMUHypervisor.QMPMonitor.Network, mc.QEMUHypervisor.QMPMonitor.Address.GetPath(), mc.QEMUHypervisor.QMPMonitor.Timeout)
	if err != nil {
		// If an improper cleanup was done and the socketmonitor was not deleted,
		// it can appear as though the machine state is not stopped.  Check for ECONNREFUSED
		// almost assures us that the vm is stopped.
		if errors.Is(err, syscall.ECONNREFUSED) {
			return define.Stopped, nil
		}
		return "", err
	}
	if err := monitor.Connect(); err != nil {
		// There is a case where if we stop the same vm (from running) two
		// consecutive times we can get an econnreset when trying to get the
		// state
		if errors.Is(err, syscall.ECONNRESET) {
			// try again
			logrus.Debug("received ECCONNRESET from QEMU monitor; trying again")
			secondTry := monitor.Connect()
			if errors.Is(secondTry, io.EOF) {
				return define.Stopped, nil
			}
			if secondTry != nil {
				logrus.Debugf("second attempt to connect to QEMU monitor failed")
				return "", secondTry
			}
		}

		return "", err
	}
	defer func() {
		if err := monitor.Disconnect(); err != nil {
			logrus.Error(err)
		}
	}()
	// If there is a monitor, let's see if we can query state
	return q.checkStatus(monitor)
}
