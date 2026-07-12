//go:build !windows
// +build !windows

package winquit

import (
	"fmt"
	"time"
)

func requestQuit(pid int) error {
	return fmt.Errorf("not implemented on non-Windows")
}

func quitProcess(pid int, waitNicely time.Duration) error {
	return fmt.Errorf("not implemented on non-Windows")
}
