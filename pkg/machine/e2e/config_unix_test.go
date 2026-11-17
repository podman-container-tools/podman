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

var (
	fakeImagePath = os.DevNull
	gvproxy       = "gvproxy"
)

func pgrep(n string) (string, error) {
	out, err := exec.Command("pgrep", n).Output()
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

func initPlatform()    {}
func cleanupPlatform() {}

// withFakeImage should be used in tests where the machine is
// initialized (or not) but never started.  It is intended
// to speed up CI by not processing our large machine files.
func (i *initMachine) withFakeImage(_ *machineTestBuilder) *initMachine {
	i.image = fakeImagePath
	return i
}
