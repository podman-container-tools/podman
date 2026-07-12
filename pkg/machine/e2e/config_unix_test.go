//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package e2e_test

import (
	"os"
	"os/exec"
)

var fakeImagePath string = os.DevNull

func pgrep(_ string) (string, error) {
	out, err := exec.Command("pgrep", "gvproxy").Output()
	return string(out), err
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
