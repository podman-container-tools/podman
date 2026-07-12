// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

#define STACK_SIZE 160
#define PTR_ADDRESS (STACK_SIZE - 4)

// syscall15X calls a function in libc on behalf of the syscall package.
// syscall15X takes a pointer to a struct like:
// struct {
//	fn    uintptr
//	a1    uintptr
//  ...
//	a32   uintptr
//	f1    uintptr
//	...
//	f16   uintptr
//	arm64_r8 uintptr
// }
// syscall15X must be called on the g0 stack with the
// C calling convention (use libcCall).
//
// On i386 System V ABI, all arguments are passed on the stack.
// Return value is in EAX (and EDX for 64-bit values).
GLOBL ·syscall15XABI0(SB), NOPTR|RODATA, $4
DATA ·syscall15XABI0(SB)/4, $syscall15X(SB)
TEXT syscall15X(SB), NOSPLIT|NOFRAME, $0-0
	// Called via C calling convention: argument pointer at 4(SP)
	// NOT via Go calling convention
	// On i386, the first argument is at 4(SP) after CALL pushes return address
	MOVL 4(SP), AX // get pointer to syscall15Args

	// Save callee-saved registers
	PUSHL BP
	PUSHL BX
	PUSHL SI
	PUSHL DI

	MOVL AX, BX // save args pointer in BX

	// Allocate stack space for C function arguments
	// i386 SysV: all 32 args on stack = 32 * 4 = 128 bytes
	// Plus 16 bytes for alignment and local storage
	SUBL $STACK_SIZE, SP
	MOVL BX, PTR_ADDRESS(SP) // save args pointer

	// Load function pointer
	MOVL syscall15Args_fn(BX), AX
	MOVL AX, (PTR_ADDRESS-4)(SP)  // save fn pointer

	// Push all integer arguments onto the stack (a1-a32)
	// i386 SysV ABI: arguments pushed right-to-left, but we're
	// setting up the stack from low to high addresses
	MOVL syscall15Args_a1(BX), AX
	MOVL AX, 0(SP)
	MOVL syscall15Args_a2(BX), AX
	MOVL AX, 4(SP)
	MOVL syscall15Args_a3(BX), AX
	MOVL AX, 8(SP)
	MOVL syscall15Args_a4(BX), AX
	MOVL AX, 12(SP)
	MOVL syscall15Args_a5(BX), AX
	MOVL AX, 16(SP)
	MOVL syscall15Args_a6(BX), AX
	MOVL AX, 20(SP)
	MOVL syscall15Args_a7(BX), AX
	MOVL AX, 24(SP)
	MOVL syscall15Args_a8(BX), AX
	MOVL AX, 28(SP)
	MOVL syscall15Args_a9(BX), AX
	MOVL AX, 32(SP)
	MOVL syscall15Args_a10(BX), AX
	MOVL AX, 36(SP)
	MOVL syscall15Args_a11(BX), AX
	MOVL AX, 40(SP)
	MOVL syscall15Args_a12(BX), AX
	MOVL AX, 44(SP)
	MOVL syscall15Args_a13(BX), AX
	MOVL AX, 48(SP)
	MOVL syscall15Args_a14(BX), AX
	MOVL AX, 52(SP)
	MOVL syscall15Args_a15(BX), AX
	MOVL AX, 56(SP)
	MOVL syscall15Args_a16(BX), AX
	MOVL AX, 60(SP)
	MOVL syscall15Args_a17(BX), AX
	MOVL AX, 64(SP)
	MOVL syscall15Args_a18(BX), AX
	MOVL AX, 68(SP)
	MOVL syscall15Args_a19(BX), AX
	MOVL AX, 72(SP)
	MOVL syscall15Args_a20(BX), AX
	MOVL AX, 76(SP)
	MOVL syscall15Args_a21(BX), AX
	MOVL AX, 80(SP)
	MOVL syscall15Args_a22(BX), AX
	MOVL AX, 84(SP)
	MOVL syscall15Args_a23(BX), AX
	MOVL AX, 88(SP)
	MOVL syscall15Args_a24(BX), AX
	MOVL AX, 92(SP)
	MOVL syscall15Args_a25(BX), AX
	MOVL AX, 96(SP)
	MOVL syscall15Args_a26(BX), AX
	MOVL AX, 100(SP)
	MOVL syscall15Args_a27(BX), AX
	MOVL AX, 104(SP)
	MOVL syscall15Args_a28(BX), AX
	MOVL AX, 108(SP)
	MOVL syscall15Args_a29(BX), AX
	MOVL AX, 112(SP)
	MOVL syscall15Args_a30(BX), AX
	MOVL AX, 116(SP)
	MOVL syscall15Args_a31(BX), AX
	MOVL AX, 120(SP)
	MOVL syscall15Args_a32(BX), AX
	MOVL AX, 124(SP)

	// Call the C function
	MOVL (PTR_ADDRESS-4)(SP), AX
	CALL AX

	// Get args pointer back and save results
	MOVL PTR_ADDRESS(SP), BX
	MOVL AX, syscall15Args_a1(BX) // return value r1
	MOVL DX, syscall15Args_a2(BX) // return value r2 (for 64-bit returns)

	// Save x87 FPU return value (ST0) to f1 field
	// On i386 System V ABI, float/double returns are in ST(0)
	// We save as float64 (8 bytes) to preserve precision
	FMOVDP F0, syscall15Args_f1(BX)

	// Clean up stack
	ADDL $STACK_SIZE, SP

	// Restore callee-saved registers
	POPL DI
	POPL SI
	POPL BX
	POPL BP

	RET
