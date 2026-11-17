package main

import (
	"fmt"
	"syscall"

	"github.com/sirupsen/logrus"
)

func setRLimits() error {
	rlimits := new(syscall.Rlimit)
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		return fmt.Errorf("getting rlimits: %w", err)
	}
	rlimits.Cur = rlimits.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, rlimits); err != nil {
		return fmt.Errorf("setting new rlimits: %w", err)
	}
	return nil
}

func setUMask() {
	// Be sure we can create directories with 0755 mode.
	syscall.Umask(0o022)
}

func earlyInitHook() {
	if err := setRLimits(); err != nil {
		logrus.Errorf("Failed to set rlimits: %v", err)
	}

	setUMask()
}
