// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build linux

package purego

import (
	"unsafe"
)

const (
	maxArgs = 15
)

type syscallArgs struct {
	fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr
	f1, f2, f3, f4, f5, f6, f7, f8                                       uintptr
	arm64_r8                                                             uintptr
}

func syscall_SyscallN(fn uintptr, sysargs []uintptr, floats []uintptr, r8 uintptr) *syscallArgs {
	s := thePool.Get().(*syscallArgs)
	*s = syscallArgs{
		fn: fn,
		a1: sysargs[0], a2: sysargs[1], a3: sysargs[2], a4: sysargs[3],
		a5: sysargs[4], a6: sysargs[5], a7: sysargs[6], a8: sysargs[7],
		a9: sysargs[8], a10: sysargs[9], a11: sysargs[10], a12: sysargs[11],
		a13: sysargs[12], a14: sysargs[13], a15: sysargs[14],
		f1: floats[0], f2: floats[1], f3: floats[2], f4: floats[3],
		f5: floats[4], f6: floats[5], f7: floats[6], f8: floats[7],
		arm64_r8: r8,
	}
	runtime_cgocall(syscallXABI0, unsafe.Pointer(s))
	return s
}

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// There is an internal maximum number of arguments that SyscallN can take. It panics
// when the maximum is exceeded. It returns the result. This file builds only for
// linux/ppc64le, where the trampoline does not capture errno, so err is always 0.
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

	var tmp [maxArgs]uintptr
	copy(tmp[:], args)
	var floats [maxArgs]uintptr
	copy(floats[:], tmp[:])
	s := syscall_SyscallN(fn, tmp[:], floats[:], 0)
	defer thePool.Put(s)
	return s.a1, s.a2, s.a3
}
