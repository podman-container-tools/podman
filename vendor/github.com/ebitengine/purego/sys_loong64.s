// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

//go:build linux

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
	// push structure pointer
	SUBV $STACK_SIZE, R3
	MOVV R4, PTR_ADDRESS(R3)
	MOVV R4, R13

	MOVD syscallArgs_f1(R13), F0 // f1
	MOVD syscallArgs_f2(R13), F1 // f2
	MOVD syscallArgs_f3(R13), F2 // f3
	MOVD syscallArgs_f4(R13), F3 // f4
	MOVD syscallArgs_f5(R13), F4 // f5
	MOVD syscallArgs_f6(R13), F5 // f6
	MOVD syscallArgs_f7(R13), F6 // f7
	MOVD syscallArgs_f8(R13), F7 // f8

	MOVV syscallArgs_a1(R13), R4  // a1
	MOVV syscallArgs_a2(R13), R5  // a2
	MOVV syscallArgs_a3(R13), R6  // a3
	MOVV syscallArgs_a4(R13), R7  // a4
	MOVV syscallArgs_a5(R13), R8  // a5
	MOVV syscallArgs_a6(R13), R9  // a6
	MOVV syscallArgs_a7(R13), R10 // a7
	MOVV syscallArgs_a8(R13), R11 // a8

	// push a9-a15 onto stack
	MOVV syscallArgs_a9(R13), R12
	MOVV R12, 0(R3)
	MOVV syscallArgs_a10(R13), R12
	MOVV R12, 8(R3)
	MOVV syscallArgs_a11(R13), R12
	MOVV R12, 16(R3)
	MOVV syscallArgs_a12(R13), R12
	MOVV R12, 24(R3)
	MOVV syscallArgs_a13(R13), R12
	MOVV R12, 32(R3)
	MOVV syscallArgs_a14(R13), R12
	MOVV R12, 40(R3)
	MOVV syscallArgs_a15(R13), R12
	MOVV R12, 48(R3)
	MOVV syscallArgs_a16(R13), R12
	MOVV R12, 56(R3)
	MOVV syscallArgs_a17(R13), R12
	MOVV R12, 64(R3)
	MOVV syscallArgs_a18(R13), R12
	MOVV R12, 72(R3)
	MOVV syscallArgs_a19(R13), R12
	MOVV R12, 80(R3)
	MOVV syscallArgs_a20(R13), R12
	MOVV R12, 88(R3)
	MOVV syscallArgs_a21(R13), R12
	MOVV R12, 96(R3)
	MOVV syscallArgs_a22(R13), R12
	MOVV R12, 104(R3)
	MOVV syscallArgs_a23(R13), R12
	MOVV R12, 112(R3)
	MOVV syscallArgs_a24(R13), R12
	MOVV R12, 120(R3)
	MOVV syscallArgs_a25(R13), R12
	MOVV R12, 128(R3)
	MOVV syscallArgs_a26(R13), R12
	MOVV R12, 136(R3)
	MOVV syscallArgs_a27(R13), R12
	MOVV R12, 144(R3)
	MOVV syscallArgs_a28(R13), R12
	MOVV R12, 152(R3)
	MOVV syscallArgs_a29(R13), R12
	MOVV R12, 160(R3)
	MOVV syscallArgs_a30(R13), R12
	MOVV R12, 168(R3)
	MOVV syscallArgs_a31(R13), R12
	MOVV R12, 176(R3)
	MOVV syscallArgs_a32(R13), R12
	MOVV R12, 184(R3)

	MOVV syscallArgs_fn(R13), R12
	JAL  (R12)

	// pop structure pointer
	MOVV PTR_ADDRESS(R3), R13
	ADDV $STACK_SIZE, R3

	// save R4, R5
	MOVV R4, syscallArgs_a1(R13)
	MOVV R5, syscallArgs_a2(R13)
	MOVV R0, syscallArgs_a3(R13) // no errno here: clear the input argument

	// save f0-f3
	MOVD F0, syscallArgs_f1(R13)
	MOVD F1, syscallArgs_f2(R13)
	MOVD F2, syscallArgs_f3(R13)
	MOVD F3, syscallArgs_f4(R13)
	RET
