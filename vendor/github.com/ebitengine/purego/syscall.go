// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build !386 && !arm && !ppc64le && (darwin || freebsd || linux || netbsd || windows)

package purego

import (
	"runtime"
	"structs"
	"unsafe"
)

const (
	maxArgs = 32
)

// syscallArgs is the argument block handed to the syscall15 trampoline. On
// platforms that go through internal/cgo it is passed to C as a
// struct syscallArgs, so its layout must match the C ABI.
//
// The trampoline reports its results through this block: the return values go
// to a1 and a2 and, where the platform captures errno, the error code goes to
// a3. A trampoline that cannot capture errno must clear a3.
type syscallArgs struct {
	_ structs.HostLayout

	fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15                uintptr
	a16, a17, a18, a19, a20, a21, a22, a23, a24, a25, a26, a27, a28, a29, a30, a31, a32 uintptr
	f1, f2, f3, f4, f5, f6, f7, f8                                                      uintptr
	arm64_r8                                                                            uintptr
}

func syscall_SyscallN(fn uintptr, sysargs []uintptr, floats []uintptr, r8 uintptr) *syscallArgs {
	s := thePool.Get().(*syscallArgs)
	*s = syscallArgs{
		fn: fn,
		a1: sysargs[0], a2: sysargs[1], a3: sysargs[2], a4: sysargs[3],
		a5: sysargs[4], a6: sysargs[5], a7: sysargs[6], a8: sysargs[7],
		a9: sysargs[8], a10: sysargs[9], a11: sysargs[10], a12: sysargs[11],
		a13: sysargs[12], a14: sysargs[13], a15: sysargs[14], a16: sysargs[15],
		a17: sysargs[16], a18: sysargs[17], a19: sysargs[18], a20: sysargs[19],
		a21: sysargs[20], a22: sysargs[21], a23: sysargs[22], a24: sysargs[23],
		a25: sysargs[24], a26: sysargs[25], a27: sysargs[26], a28: sysargs[27],
		a29: sysargs[28], a30: sysargs[29], a31: sysargs[30], a32: sysargs[31],
		f1: floats[0], f2: floats[1], f3: floats[2], f4: floats[3],
		f5: floats[4], f6: floats[5], f7: floats[6], f8: floats[7],
		arm64_r8: r8,
	}
	runtime_cgocall(syscallXABI0, unsafe.Pointer(s))
	return s
}

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// There is an internal maximum number of arguments that SyscallN can take. It panics
// when the maximum is exceeded. It returns the result and GetLastError on Windows or the errno on
// platforms that capture it, otherwise 0.
//
// In order to call this function properly make sure to follow all the rules specified in [unsafe.Pointer]
// especially point 4.
//
// NOTE: SyscallN does not properly call functions that have both integer and float parameters.
// See discussion comment https://github.com/ebiten/purego/pull/1#issuecomment-1128057607
// for an explanation of why that is.
//
// On amd64, if there are more than 8 floats the 9th and so on will be placed incorrectly on the
// stack.
//
// The pragma go:nosplit is not needed at this function declaration because it uses go:uintptrescapes
// which forces all the objects that the uintptrs point to onto the heap where a stack split won't affect
// their memory location.
//
//go:uintptrescapes
func SyscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	if fn == 0 {
		panic("purego: fn is nil")
	}
	if len(args) > maxArgs {
		panic("purego: too many arguments to SyscallN")
	}

	// Windows uses syscall.SyscallN in syscall_windows.go.
	if runtime.GOOS == "windows" {
		return syscall_syscallN(fn, args...)
	}

	// add padding so there is no out-of-bounds slicing
	var tmp [maxArgs]uintptr
	copy(tmp[:], args)
	var floats [maxArgs]uintptr
	copy(floats[:], tmp[:])
	s := syscall_SyscallN(fn, tmp[:], floats[:], 0)
	defer thePool.Put(s)
	return s.a1, s.a2, s.a3
}
