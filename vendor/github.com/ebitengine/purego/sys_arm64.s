// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || freebsd || linux || netbsd || windows

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

#define STACK_SIZE 208
#define PTR_ADDRESS (STACK_SIZE - 8)

// syscallX calls a function in libc on behalf of the syscall package.
// syscallX takes a pointer to a struct like:
// struct {
//	fn    uintptr
//	a1    uintptr
//	a2    uintptr
//	a3    uintptr
//	a4    uintptr
//	a5    uintptr
//	a6    uintptr
//	a7    uintptr
//	a8    uintptr
//	a9    uintptr
//	a10    uintptr
//	a11    uintptr
//	a12    uintptr
//	a13    uintptr
//	a14    uintptr
//	a15    uintptr
//	r1    uintptr
//	r2    uintptr
//	err   uintptr
// }
// syscallX must be called on the g0 stack with the
// C calling convention (use libcCall).
GLOBL ·syscallXABI0(SB), NOPTR|RODATA, $8
DATA ·syscallXABI0(SB)/8, $syscallX(SB)
TEXT syscallX(SB), NOSPLIT, $0
	SUB  $STACK_SIZE, RSP     // push structure pointer
	MOVD R0, PTR_ADDRESS(RSP)
	MOVD R0, R9

	FMOVD syscallArgs_f1(R9), F0 // f1
	FMOVD syscallArgs_f2(R9), F1 // f2
	FMOVD syscallArgs_f3(R9), F2 // f3
	FMOVD syscallArgs_f4(R9), F3 // f4
	FMOVD syscallArgs_f5(R9), F4 // f5
	FMOVD syscallArgs_f6(R9), F5 // f6
	FMOVD syscallArgs_f7(R9), F6 // f7
	FMOVD syscallArgs_f8(R9), F7 // f8

	MOVD syscallArgs_a1(R9), R0       // a1
	MOVD syscallArgs_a2(R9), R1       // a2
	MOVD syscallArgs_a3(R9), R2       // a3
	MOVD syscallArgs_a4(R9), R3       // a4
	MOVD syscallArgs_a5(R9), R4       // a5
	MOVD syscallArgs_a6(R9), R5       // a6
	MOVD syscallArgs_a7(R9), R6       // a7
	MOVD syscallArgs_a8(R9), R7       // a8
	MOVD syscallArgs_arm64_r8(R9), R8 // r8

	MOVD syscallArgs_a9(R9), R10
	MOVD R10, 0(RSP)                // push a9 onto stack
	MOVD syscallArgs_a10(R9), R10
	MOVD R10, 8(RSP)                // push a10 onto stack
	MOVD syscallArgs_a11(R9), R10
	MOVD R10, 16(RSP)               // push a11 onto stack
	MOVD syscallArgs_a12(R9), R10
	MOVD R10, 24(RSP)               // push a12 onto stack
	MOVD syscallArgs_a13(R9), R10
	MOVD R10, 32(RSP)               // push a13 onto stack
	MOVD syscallArgs_a14(R9), R10
	MOVD R10, 40(RSP)               // push a14 onto stack
	MOVD syscallArgs_a15(R9), R10
	MOVD R10, 48(RSP)               // push a15 onto stack
	MOVD syscallArgs_a16(R9), R10
	MOVD R10, 56(RSP)               // push a16 onto stack
	MOVD syscallArgs_a17(R9), R10
	MOVD R10, 64(RSP)               // push a17 onto stack
	MOVD syscallArgs_a18(R9), R10
	MOVD R10, 72(RSP)               // push a18 onto stack
	MOVD syscallArgs_a19(R9), R10
	MOVD R10, 80(RSP)               // push a19 onto stack
	MOVD syscallArgs_a20(R9), R10
	MOVD R10, 88(RSP)               // push a20 onto stack
	MOVD syscallArgs_a21(R9), R10
	MOVD R10, 96(RSP)               // push a21 onto stack
	MOVD syscallArgs_a22(R9), R10
	MOVD R10, 104(RSP)              // push a22 onto stack
	MOVD syscallArgs_a23(R9), R10
	MOVD R10, 112(RSP)              // push a23 onto stack
	MOVD syscallArgs_a24(R9), R10
	MOVD R10, 120(RSP)              // push a24 onto stack
	MOVD syscallArgs_a25(R9), R10
	MOVD R10, 128(RSP)              // push a25 onto stack
	MOVD syscallArgs_a26(R9), R10
	MOVD R10, 136(RSP)              // push a26 onto stack
	MOVD syscallArgs_a27(R9), R10
	MOVD R10, 144(RSP)              // push a27 onto stack
	MOVD syscallArgs_a28(R9), R10
	MOVD R10, 152(RSP)              // push a28 onto stack
	MOVD syscallArgs_a29(R9), R10
	MOVD R10, 160(RSP)              // push a29 onto stack
	MOVD syscallArgs_a30(R9), R10
	MOVD R10, 168(RSP)              // push a30 onto stack
	MOVD syscallArgs_a31(R9), R10
	MOVD R10, 176(RSP)              // push a31 onto stack
	MOVD syscallArgs_a32(R9), R10
	MOVD R10, 184(RSP)              // push a32 onto stack

	MOVD syscallArgs_fn(R9), R10 // fn
	BL   (R10)

	MOVD PTR_ADDRESS(RSP), R2 // pop structure pointer
	ADD  $STACK_SIZE, RSP

	MOVD  R0, syscallArgs_a1(R2) // save r1
	MOVD  R1, syscallArgs_a2(R2) // save r3
	FMOVD F0, syscallArgs_f1(R2) // save f0
	FMOVD F1, syscallArgs_f2(R2) // save f1
	FMOVD F2, syscallArgs_f3(R2) // save f2
	FMOVD F3, syscallArgs_f4(R2) // save f3

#ifdef GOOS_darwin
	BL   purego_error(SB)
	MOVD (R0), R0
	MOVD R0, syscallArgs_a3(R2) // save errno
#else
	MOVD $0, syscallArgs_a3(R2) // no errno here: clear the input argument
#endif
	RET
