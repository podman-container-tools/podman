// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build !386 && !arm && (darwin || freebsd || linux || netbsd || windows)

package purego

// CDecl marks a function as being called using the __cdecl calling convention as defined in
// the [MSDocs] when passed to NewCallback. It must be the first argument to the function.
// This is only useful on 386 Windows, but it is safe to use on other platforms.
//
// [MSDocs]: https://learn.microsoft.com/en-us/cpp/cpp/cdecl?view=msvc-170
type CDecl struct{}

const (
	maxArgs = 15
)

type syscall15Args struct {
	fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr
	f1, f2, f3, f4, f5, f6, f7, f8                                       uintptr
	arm64_r8                                                             uintptr
}

func (s *syscall15Args) Set(fn uintptr, ints []uintptr, floats []uintptr, r8 uintptr) {
	s.fn = fn
	s.a1 = ints[0]
	s.a2 = ints[1]
	s.a3 = ints[2]
	s.a4 = ints[3]
	s.a5 = ints[4]
	s.a6 = ints[5]
	s.a7 = ints[6]
	s.a8 = ints[7]
	s.a9 = ints[8]
	s.a10 = ints[9]
	s.a11 = ints[10]
	s.a12 = ints[11]
	s.a13 = ints[12]
	s.a14 = ints[13]
	s.a15 = ints[14]
	s.f1 = floats[0]
	s.f2 = floats[1]
	s.f3 = floats[2]
	s.f4 = floats[3]
	s.f5 = floats[4]
	s.f6 = floats[5]
	s.f7 = floats[6]
	s.f8 = floats[7]
	s.arm64_r8 = r8
}

// SyscallN takes fn, a C function pointer and a list of arguments as uintptr.
// There is an internal maximum number of arguments that SyscallN can take. It panics
// when the maximum is exceeded. It returns the result and the libc error code if there is one.
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
	// add padding so there is no out-of-bounds slicing
	var tmp [maxArgs]uintptr
	copy(tmp[:], args)
	return syscall_syscall15X(fn, tmp[0], tmp[1], tmp[2], tmp[3], tmp[4], tmp[5], tmp[6], tmp[7], tmp[8], tmp[9], tmp[10], tmp[11], tmp[12], tmp[13], tmp[14])
}
