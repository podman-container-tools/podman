//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package e2e_test

import (
	"context"
	"os"
	"os/exec"
)

var (
	fakeImagePath = os.DevNull
	gvproxy       = "gvproxy"
)

func pgrep(ctx context.Context, n string) (string, error) {
	out, err := exec.CommandContext(ctx, "pgrep", n).Output()
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
