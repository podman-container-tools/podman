//go:build windows

package terminal

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// SetConsole switches the windows terminal mode to be able to handle colors, etc
func SetConsole() error {
	if err := setConsoleMode(windows.Stdout, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return err
	}
	if err := setConsoleMode(windows.Stderr, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return err
	}
	if err := setConsoleMode(windows.Stdin, windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return err
	}
	return nil
}

func setConsoleMode(handle windows.Handle, flags uint32) error {
	var mode uint32
	err := windows.GetConsoleMode(handle, &mode)
	if err != nil {
		//nolint:nilerr
		return nil // not a terminal
	}
	if err := windows.SetConsoleMode(handle, mode|flags); err != nil {
		// In similar code, it is not considered an error if we cannot set the
		// console mode.  Following same line of thinking here.
		logrus.WithError(err).Debug("Failed to set console mode for cli")
	}

	return nil
}
