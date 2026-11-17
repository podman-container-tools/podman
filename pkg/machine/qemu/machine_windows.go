//go:build windows

package qemu

import (
	"bytes"
	"fmt"

	"go.podman.io/podman/v6/pkg/machine"
)

func isProcessAlive(pid int) bool {
	return checkProcessStatus("process", pid, nil) == nil
}

func checkProcessStatus(processHint string, pid int, stderrBuf *bytes.Buffer) error {
	active, exitCode := machine.GetProcessState(pid)
	if !active {
		if stderrBuf != nil {
			return fmt.Errorf("%s exited unexpectedly, exit code: %d stderr: %s", processHint, exitCode, stderrBuf.String())
		} else {
			return fmt.Errorf("%s exited unexpectedly, exit code: %d", processHint, exitCode)
		}
	}
	return nil
}

func sigKill(_ int) error {
	return nil
}

func findProcess(_ int) (int, error) {
	return -1, nil
}
