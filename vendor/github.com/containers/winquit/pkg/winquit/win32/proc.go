//go:build windows
// +build windows

package win32

import (
	"fmt"
	"syscall"
)

const (
	MAXIMUM_ALLOWED = 0x02000000
)

var (
	procOpenProcess     = kernel32.NewProc("OpenProcess")
	procCloseHandle     = kernel32.NewProc("CloseHandle")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

func OpenProcess(pid uint32) (syscall.Handle, error) {
	ret, _, err :=
		procOpenProcess.Call( // HANDLE OpenProcess()
			MAXIMUM_ALLOWED, //         [in] DWORD dwDesiredAccess,
			0,               //         [in] BOOL  bInheritHandle,
			uintptr(pid),    //         [in] DWORD dwProcessId
		)

	if ret == 0 {
		return 0, err
	}

	return syscall.Handle(ret), nil
}

func CloseHandle(handle syscall.Handle) error {
	ret, _, err :=
		procCloseHandle.Call( // BOOL CloseHandle()
			uintptr(handle), //       [in] HANDLE hObject
		)
	if ret != 0 {
		return fmt.Errorf("error closing handle: %w", err)
	}

	return nil
}

func GetProcThreads(pid uint32) ([]uint, error) {
	process, err := OpenProcess(pid)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = CloseHandle(process)
	}()

	return GetProcThreadIds(process)
}
