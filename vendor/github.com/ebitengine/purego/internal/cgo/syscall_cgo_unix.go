// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build freebsd || (linux && !(386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64)) || netbsd

package cgo

// this file is placed inside internal/cgo and not package purego
// because Cgo and assembly files can't be in the same package.

/*
#cgo !netbsd LDFLAGS: -ldl

#include <stdint.h>
#include <dlfcn.h>
#include <errno.h>
#include <assert.h>

typedef struct syscallArgs {
	uintptr_t fn;
	uintptr_t a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15;
	uintptr_t a16, a17, a18, a19, a20, a21, a22, a23, a24, a25, a26, a27, a28, a29, a30, a31, a32;
	uintptr_t f1, f2, f3, f4, f5, f6, f7, f8;
	uintptr_t arm64_r8;
} syscallArgs;

void syscall15(struct syscallArgs *args) {
	assert((args->f1|args->f2|args->f3|args->f4|args->f5|args->f6|args->f7|args->f8) == 0);
	uintptr_t (*func_name)(uintptr_t a1, uintptr_t a2, uintptr_t a3, uintptr_t a4, uintptr_t a5, uintptr_t a6,
		uintptr_t a7, uintptr_t a8, uintptr_t a9, uintptr_t a10, uintptr_t a11, uintptr_t a12,
		uintptr_t a13, uintptr_t a14, uintptr_t a15, uintptr_t a16, uintptr_t a17, uintptr_t a18,
		uintptr_t a19, uintptr_t a20, uintptr_t a21, uintptr_t a22, uintptr_t a23, uintptr_t a24,
		uintptr_t a25, uintptr_t a26, uintptr_t a27, uintptr_t a28, uintptr_t a29, uintptr_t a30,
		uintptr_t a31, uintptr_t a32);
	*(void**)(&func_name) = (void*)(args->fn);
	uintptr_t r1 = func_name(args->a1,args->a2,args->a3,args->a4,args->a5,args->a6,args->a7,args->a8,args->a9,
		args->a10,args->a11,args->a12,args->a13,args->a14,args->a15,args->a16,args->a17,args->a18,
		args->a19,args->a20,args->a21,args->a22,args->a23,args->a24,args->a25,args->a26,args->a27,
		args->a28,args->a29,args->a30,args->a31,args->a32);
	args->a1 = r1;
	args->a3 = errno;
}

*/
import "C"
import "unsafe"

// assign purego.syscallXABI0 to the C version of this function.
var SyscallXABI0 = unsafe.Pointer(C.syscall15)
