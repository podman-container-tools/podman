// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

// Stack usage:
//  0(SP)  - 56(SP):  stack args a9-a15 (7 * 8 bytes = 56)
// 56(SP)  - 64(SP):  saved RA (x1)
// 64(SP)  - 72(SP):  saved X9 (s1)
// 72(SP)  - 80(SP):  saved X18 (s2)
// 80(SP)  - 88(SP):  saved args pointer (original X10)
// 88(SP)  - 96(SP):  padding
#define STACK_SIZE 96
#define SAVE_RA    56
#define SAVE_X9    64
#define SAVE_X18   72
#define SAVE_ARGP  80

GLOBL ·syscall15XABI0(SB), NOPTR|RODATA, $8
DATA ·syscall15XABI0(SB)/8, $syscall15X(SB)

TEXT syscall15X(SB), NOSPLIT, $0
	// Allocate stack frame (keeps 16-byte alignment)
	SUB $STACK_SIZE, SP

	// Save callee-saved regs we clobber + return address
	MOV X1, SAVE_RA(SP)
	MOV X9, SAVE_X9(SP)
	MOV X18, SAVE_X18(SP)

	// Save original args pointer (in a0/X10)
	MOV X10, SAVE_ARGP(SP)

	// X9 := args pointer (syscall15Args*)
	MOV X10, X9

	// Load float args into fa0-fa7 (F10-F17)
	MOVD syscall15Args_f1(X9), F10
	MOVD syscall15Args_f2(X9), F11
	MOVD syscall15Args_f3(X9), F12
	MOVD syscall15Args_f4(X9), F13
	MOVD syscall15Args_f5(X9), F14
	MOVD syscall15Args_f6(X9), F15
	MOVD syscall15Args_f7(X9), F16
	MOVD syscall15Args_f8(X9), F17

	// Load integer args into a0-a7 (X10-X17)
	MOV syscall15Args_a1(X9), X10
	MOV syscall15Args_a2(X9), X11
	MOV syscall15Args_a3(X9), X12
	MOV syscall15Args_a4(X9), X13
	MOV syscall15Args_a5(X9), X14
	MOV syscall15Args_a6(X9), X15
	MOV syscall15Args_a7(X9), X16
	MOV syscall15Args_a8(X9), X17

	// Spill a9-a15 onto the stack (C ABI)
	MOV syscall15Args_a9(X9), X18
	MOV X18, 0(SP)
	MOV syscall15Args_a10(X9), X18
	MOV X18, 8(SP)
	MOV syscall15Args_a11(X9), X18
	MOV X18, 16(SP)
	MOV syscall15Args_a12(X9), X18
	MOV X18, 24(SP)
	MOV syscall15Args_a13(X9), X18
	MOV X18, 32(SP)
	MOV syscall15Args_a14(X9), X18
	MOV X18, 40(SP)
	MOV syscall15Args_a15(X9), X18
	MOV X18, 48(SP)

	// Call fn
	// IMPORTANT: preserve RA across this call (we saved it above)
	MOV  syscall15Args_fn(X9), X18
	CALL X18

	// Restore args pointer (syscall15Args*) for storing results
	MOV SAVE_ARGP(SP), X9

	// Store results back
	MOV X10, syscall15Args_a1(X9)
	MOV X11, syscall15Args_a2(X9)

	// Store back float return regs if used by your ABI contract
	MOVD F10, syscall15Args_f1(X9)
	MOVD F11, syscall15Args_f2(X9)
	MOVD F12, syscall15Args_f3(X9)
	MOVD F13, syscall15Args_f4(X9)

	// Restore callee-saved regs and return address
	MOV SAVE_X18(SP), X18
	MOV SAVE_X9(SP), X9
	MOV SAVE_RA(SP), X1

	ADD $STACK_SIZE, SP
	RET
