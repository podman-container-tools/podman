//go:build windows

package sysinfo

import (
	"math/bits"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	getCurrentProcess      = kernel32.NewProc("GetCurrentProcess")
	getProcessAffinityMask = kernel32.NewProc("GetProcessAffinityMask")
)

func numCPU() int {
	// Gets the affinity mask for a process
	var mask, sysmask uintptr
	currentProcess, _, _ := getCurrentProcess.Call()
	ret, _, _ := getProcessAffinityMask.Call(currentProcess, uintptr(unsafe.Pointer(&mask)), uintptr(unsafe.Pointer(&sysmask)))
	if ret == 0 {
		return 0
	}
	return bits.OnesCount64(uint64(mask))
}
