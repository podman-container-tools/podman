//go:build windows
// +build windows

package win32

import (
	"syscall"
)

const (
	ERROR_NO_MORE_ITEMS = 259
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
)
