// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build (386 || arm) && (freebsd || linux || netbsd || windows)

package purego

// CDecl marks a function as being called using the __cdecl calling convention as defined in
// the [MSDocs] when passed to NewCallback. It must be the first argument to the function.
// This is only useful on 386 Windows, but it is safe to use on other platforms.
//
// [MSDocs]: https://learn.microsoft.com/en-us/cpp/cpp/cdecl?view=msvc-170
type CDecl struct{}

const (
	maxArgs = 32
)

type syscall15Args struct {
	fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15                uintptr
	a16, a17, a18, a19, a20, a21, a22, a23, a24, a25, a26, a27, a28, a29, a30, a31, a32 uintptr
	f1, f2, f3, f4, f5, f6, f7, f8, f9, f10, f11, f12, f13, f14, f15, f16               uintptr
	arm64_r8                                                                            uintptr
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
	s.a16 = ints[15]
	s.a17 = ints[16]
	s.a18 = ints[17]
	s.a19 = ints[18]
	s.a20 = ints[19]
	s.a21 = ints[20]
	s.a22 = ints[21]
	s.a23 = ints[22]
	s.a24 = ints[23]
	s.a25 = ints[24]
	s.a26 = ints[25]
	s.a27 = ints[26]
	s.a28 = ints[27]
	s.a29 = ints[28]
	s.a30 = ints[29]
	s.a31 = ints[30]
	s.a32 = ints[31]
	s.f1 = floats[0]
	s.f2 = floats[1]
	s.f3 = floats[2]
	s.f4 = floats[3]
	s.f5 = floats[4]
	s.f6 = floats[5]
	s.f7 = floats[6]
	s.f8 = floats[7]
	s.f9 = floats[8]
	s.f10 = floats[9]
	s.f11 = floats[10]
	s.f12 = floats[11]
	s.f13 = floats[12]
	s.f14 = floats[13]
	s.f15 = floats[14]
	s.f16 = floats[15]
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
