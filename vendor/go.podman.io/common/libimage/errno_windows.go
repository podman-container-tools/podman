//go:build windows

package libimage

import "golang.org/x/sys/windows"

const errNoSpace = windows.ERROR_DISK_FULL
