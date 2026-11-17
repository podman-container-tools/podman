//go:build linux

package loopback

import (
	"golang.org/x/sys/unix"
)

// IOCTL consts
const (
	LoopSetFd       = unix.LOOP_SET_FD       // Deprecated: Use unix.LOOP_SET_FD directly.
	LoopCtlGetFree  = unix.LOOP_CTL_GET_FREE // Deprecated: Use unix.LOOP_CTL_GET_FREE directly.
	LoopGetStatus64 = unix.LOOP_GET_STATUS64 // Deprecated: Use unix.LOOP_GET_STATUS64 directly.
	LoopSetStatus64 = unix.LOOP_SET_STATUS64 // Deprecated: Use unix.LOOP_SET_STATUS64 directly.
	LoopClrFd       = unix.LOOP_CLR_FD       // Deprecated: Use unix.LOOP_CLR_FD directly.
	LoopSetCapacity = unix.LOOP_SET_CAPACITY // Deprecated: Use unix.LOOP_SET_CAPACITY directly.
)

// LOOP consts.
const (
	LoFlagsAutoClear = unix.LO_FLAGS_AUTOCLEAR // Deprecated: Use unix.LO_FLAGS_AUTOCLEAR directly.
	LoFlagsReadOnly  = unix.LO_FLAGS_READ_ONLY // Deprecated: Use unix.LO_FLAGS_READ_ONLY directly.
	LoFlagsPartScan  = unix.LO_FLAGS_PARTSCAN  // Deprecated: Use unix.LO_FLAGS_PARTSCAN directly.
	LoKeySize        = unix.LO_KEY_SIZE        // Deprecated: Use unix.LO_KEY_SIZE directly.
	LoNameSize       = unix.LO_NAME_SIZE       // Deprecated: Use unix.LO_NAME_SIZE directly.
)
