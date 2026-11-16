//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func pgrep(_ string) (string, error) {
	out, err := exec.Command("pgrep", "gvproxy").Output()
	return string(out), err
}

// apiProxyPID returns the PID of the host proxy serving the machine API.
func apiProxyPID(_, runtimeDir string) int {
	return gvproxyPID(runtimeDir)
}

// processExists reports whether pid is running. A killed process that has
// not been reaped yet (a zombie) does not count.
func processExists(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		// No procfs (e.g. macOS)
		return true
	}
	// The state is the first field after the command name, which ends with ")".
	fields := strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))
	return len(fields) == 0 || fields[0] != "Z"
}
