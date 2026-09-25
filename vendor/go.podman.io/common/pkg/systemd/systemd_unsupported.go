//go:build !linux

package systemd

import "errors"

func RunsOnSystemd() bool {
	return false
}

func MovePauseProcessToScope(pausePidPath string) {}

func RunUnderSystemdScope(pids []int, slice string, unitName string) error {
	return errors.New("RunUnderSystemdScope not supported on this OS")
}

func AddPidsToSystemdScope(unitName string, pids ...int) error {
	return errors.New("AddPidsToSystemdScope not supported on this OS")
}
