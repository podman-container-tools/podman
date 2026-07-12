// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

// S390X ELF ABI:
// - Integer args: R2-R6 (5 registers)
// - Float args: F0, F2, F4, F6 (4 registers, even-numbered)
// - Return: R2 (integer), F0 (float)
// - Stack pointer: R15
// - Link register: R14
// - Callee-saved: R6-R13, F8-F15 (but R6 is also used for 5th param)
//
// Stack frame layout (aligned to 8 bytes):
//   0(R15)   - back chain
//   8(R15)   - reserved
//  16(R15)   - reserved
//  ...       - register save area (R6-R15 at 48(R15))
// 160(R15)  - parameter area start (args beyond registers)
//
// We need space for:
// - 160 bytes standard frame (with register save area)
// - Stack args a6-a15 (10 * 8 = 80 bytes)
// - Saved args pointer (8 bytes)
// - Padding for alignment
// Total: 264 bytes (rounded to 8-byte alignment)

#define STACK_SIZE 264
#define STACK_ARGS 160
#define ARGP_SAVE  248

GLOBL ·syscall15XABI0(SB), NOPTR|RODATA, $8
DATA ·syscall15XABI0(SB)/8, $syscall15X(SB)

TEXT syscall15X(SB), NOSPLIT, $0
	// On entry, R2 contains the args pointer
	// Save callee-saved registers in caller's frame (per ABI)
	STMG R6, R15, 48(R15)

	// Allocate our stack frame
	MOVD R15, R1
	SUB  $STACK_SIZE, R15
	MOVD R1, 0(R15)       // back chain

	// Save args pointer
	MOVD R2, ARGP_SAVE(R15)

	// R9 := args pointer (syscall15Args*)
	MOVD R2, R9

	// Load float args into F0, F2, F4, F6 (s390x uses even-numbered FPRs)
	FMOVD syscall15Args_f1(R9), F0
	FMOVD syscall15Args_f2(R9), F2
	FMOVD syscall15Args_f3(R9), F4
	FMOVD syscall15Args_f4(R9), F6

	// Load integer args into R2-R6 (5 registers)
	MOVD syscall15Args_a1(R9), R2
	MOVD syscall15Args_a2(R9), R3
	MOVD syscall15Args_a3(R9), R4
	MOVD syscall15Args_a4(R9), R5
	MOVD syscall15Args_a5(R9), R6

	// Spill remaining args (a6-a15) onto the stack at 160(R15)
	MOVD ARGP_SAVE(R15), R9        // reload args pointer
	MOVD syscall15Args_a6(R9), R1
	MOVD R1, (STACK_ARGS+0*8)(R15)
	MOVD syscall15Args_a7(R9), R1
	MOVD R1, (STACK_ARGS+1*8)(R15)
	MOVD syscall15Args_a8(R9), R1
	MOVD R1, (STACK_ARGS+2*8)(R15)
	MOVD syscall15Args_a9(R9), R1
	MOVD R1, (STACK_ARGS+3*8)(R15)
	MOVD syscall15Args_a10(R9), R1
	MOVD R1, (STACK_ARGS+4*8)(R15)
	MOVD syscall15Args_a11(R9), R1
	MOVD R1, (STACK_ARGS+5*8)(R15)
	MOVD syscall15Args_a12(R9), R1
	MOVD R1, (STACK_ARGS+6*8)(R15)
	MOVD syscall15Args_a13(R9), R1
	MOVD R1, (STACK_ARGS+7*8)(R15)
	MOVD syscall15Args_a14(R9), R1
	MOVD R1, (STACK_ARGS+8*8)(R15)
	MOVD syscall15Args_a15(R9), R1
	MOVD R1, (STACK_ARGS+9*8)(R15)

	// Call function
	MOVD syscall15Args_fn(R9), R1
	BL   (R1)

	// Restore args pointer for storing results
	MOVD ARGP_SAVE(R15), R9

	// Store integer results back (R2, R3)
	MOVD R2, syscall15Args_a1(R9)
	MOVD R3, syscall15Args_a2(R9)

	// Store float return values (F0, F2, F4, F6)
	FMOVD F0, syscall15Args_f1(R9)
	FMOVD F2, syscall15Args_f2(R9)
	FMOVD F4, syscall15Args_f3(R9)
	FMOVD F6, syscall15Args_f4(R9)

	// Deallocate stack frame
	ADD $STACK_SIZE, R15

	// Restore callee-saved registers from caller's save area
	LMG 48(R15), R6, R15

	RET
