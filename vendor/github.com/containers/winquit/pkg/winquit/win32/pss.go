//go:build windows
// +build windows

package win32

import (
	"fmt"
	"syscall"
	"unsafe"
)

type PSS_THREAD_ENTRY struct {
	ExitStatus               uint32
	TebBaseAddress           uintptr
	ProcessId                uint32
	ThreadId                 uint32
	AffinityMask             uintptr
	Priority                 int32
	BasePriority             int32
	LastSyscallFirstArgument uintptr
	LastSyscallNumber        uint16
	CreateTime               uint64
	ExitTime                 uint64
	KernelTime               uint64
	UserTime                 uint64
	Win32StartAddress        uintptr
	CaptureTime              uint64
	Flags                    uint32
	SuspendCount             uint16
	SizeOfContextRecord      uint16
	ContextRecord            uintptr
}

const (
	PSS_CAPTURE_THREADS          = 0x00000080
	PSS_WALK_THREADS             = 3
	PSS_QUERY_THREAD_INFORMATION = 5
)

var (
	procPssCaptureSnapshot  = kernel32.NewProc("PssCaptureSnapshot")
	procPssFreeSnapshot     = kernel32.NewProc("PssFreeSnapshot")
	procPssWalkMarkerCreate = kernel32.NewProc("PssWalkMarkerCreate")
	procPssWalkMarkerFree   = kernel32.NewProc("PssWalkMarkerFree")
	procPssWalkSnapshot     = kernel32.NewProc("PssWalkSnapshot")
)

func PssCaptureSnapshot(process syscall.Handle, flags int32, contextFlags int32) (syscall.Handle, error) {
	var snapshot syscall.Handle
	ret, _, err :=
		procPssCaptureSnapshot.Call( //       DWORD PssCaptureSnapshot()
			uintptr(process),                   //  [in]           HANDLE            ProcessHandle,
			uintptr(flags),                     //  [in]           PSS_CAPTURE_FLAGS CaptureFlags,
			uintptr(contextFlags),              //  [in, optional] DWORD             ThreadContextFlags,
			uintptr(unsafe.Pointer(&snapshot)), //  [out]          HPSS              *SnapshotHandle
		)

	if ret != 0 {
		return 0, err
	}

	return snapshot, nil
}

func PssFreeSnapshot(process syscall.Handle, snapshot syscall.Handle) error {
	ret, _, _ :=
		procPssFreeSnapshot.Call( // DWORD PssFreeSnapshot()
			uintptr(process),  //          [in] HANDLE ProcessHandle,
			uintptr(snapshot), //          [in] HPSS   SnapshotHandle
		)
	if ret != 0 {
		return fmt.Errorf("error freeing snapshot: %d", ret)
	}

	return nil
}

func PssWalkMarkerCreate() (syscall.Handle, error) {
	var walkptr uintptr

	ret, _, _ :=
		procPssWalkMarkerCreate.Call( //       // DWORD PssWalkMarkerCreate()
			0,                                 //       [in, optional] PSS_ALLOCATOR const *Allocator
			uintptr(unsafe.Pointer(&walkptr)), //       [out]          HPSSWALK            *WalkMarkerHandle
		)
	if ret != 0 {
		return 0, fmt.Errorf("error creating process walker mark: %d", ret)
	}

	return syscall.Handle(walkptr), nil
}

func PssWalkMarkerFree(handle syscall.Handle) error {
	ret, _, _ :=
		procPssWalkMarkerFree.Call( // DWORD PssWalkMarkerFree()
			uintptr(handle), //              [in] HPSSWALK WalkMarkerHandle
		)
	if ret != 0 {
		return fmt.Errorf("error freeing process walker mark: %d", ret)
	}

	return nil
}

func PssWalkThreadSnapshot(snapshot syscall.Handle, marker syscall.Handle) (*PSS_THREAD_ENTRY, error) {
	var thread PSS_THREAD_ENTRY
	ret, _, err :=
		procPssWalkSnapshot.Call( //          // DWORD PssWalkSnapshot()
			uintptr(snapshot),                //       [in]  HPSS                       SnapshotHandle,
			PSS_WALK_THREADS,                 //       [in]  PSS_WALK_INFORMATION_CLASS InformationClass,
			uintptr(marker),                  //       [in]  HPSSWALK                   WalkMarkerHandle,
			uintptr(unsafe.Pointer(&thread)), //       [out] void                       *Buffer,
			unsafe.Sizeof(thread),            //       [in]  DWORD                      BufferLength
		)

	if ret == ERROR_NO_MORE_ITEMS {
		return nil, nil
	}

	if ret != 0 {
		return nil, fmt.Errorf("error waling thread snapshot: %d (%w)", ret, err)
	}

	return &thread, nil
}

func GetProcThreadIds(process syscall.Handle) ([]uint, error) {
	snapshot, err := PssCaptureSnapshot(process, PSS_CAPTURE_THREADS, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = PssFreeSnapshot(process, snapshot)
	}()

	marker, err := PssWalkMarkerCreate()
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = PssWalkMarkerFree(marker)
	}()

	var threads []uint

	for {
		thread, err := PssWalkThreadSnapshot(snapshot, marker)
		if err != nil {
			return nil, err
		}
		if thread == nil {
			break
		}

		threads = append(threads, uint(thread.ThreadId))
	}

	return threads, nil
}
