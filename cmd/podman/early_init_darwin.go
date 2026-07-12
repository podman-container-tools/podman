package main

import (
	"fmt"
	"os"
	"syscall"
)

func setRLimitsNoFile() error {
	var rLimitNoFile syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimitNoFile); err != nil {
		return fmt.Errorf("getting RLIMITS_NOFILE: %w", err)
	}
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{
		Max: rLimitNoFile.Max,
		Cur: rLimitNoFile.Max,
	})
	if err != nil {
		return fmt.Errorf("setting new RLIMITS_NOFILE: %w", err)
	}
	return nil
}

func earlyInitHook() {
	if err := setRLimitsNoFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set RLIMITS_NOFILE: %s\n", err.Error())
	}
}
