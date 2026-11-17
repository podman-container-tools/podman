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
// - Stack args a6-a32 (27 * 8 = 216 bytes)
// - Saved args pointer (8 bytes)
// Total: 384 bytes

#define STACK_SIZE 384
#define STACK_ARGS 160
#define ARGP_SAVE  376

GLOBL ·syscallXABI0(SB), NOPTR|RODATA, $8
DATA ·syscallXABI0(SB)/8, $syscallX(SB)

TEXT syscallX(SB), NOSPLIT, $0
	// On entry, R2 contains the args pointer
	// Save callee-saved registers in caller's frame (per ABI)
	STMG R6, R15, 48(R15)

	// Allocate our stack frame
	MOVD R15, R1
	SUB  $STACK_SIZE, R15
	MOVD R1, 0(R15)       // back chain

	// Save args pointer
	MOVD R2, ARGP_SAVE(R15)

	// R9 := args pointer (syscallArgs*)
	MOVD R2, R9

	// Load float args into F0, F2, F4, F6 (s390x uses even-numbered FPRs)
	FMOVD syscallArgs_f1(R9), F0
	FMOVD syscallArgs_f2(R9), F2
	FMOVD syscallArgs_f3(R9), F4
	FMOVD syscallArgs_f4(R9), F6

	// Load integer args into R2-R6 (5 registers)
	MOVD syscallArgs_a1(R9), R2
	MOVD syscallArgs_a2(R9), R3
	MOVD syscallArgs_a3(R9), R4
	MOVD syscallArgs_a4(R9), R5
	MOVD syscallArgs_a5(R9), R6

	// Spill remaining args (a6-a32) onto the stack at 160(R15)
	MOVD ARGP_SAVE(R15), R9        // reload args pointer
	MOVD syscallArgs_a6(R9), R1
	MOVD R1, (STACK_ARGS+0*8)(R15)
	MOVD syscallArgs_a7(R9), R1
	MOVD R1, (STACK_ARGS+1*8)(R15)
	MOVD syscallArgs_a8(R9), R1
	MOVD R1, (STACK_ARGS+2*8)(R15)
	MOVD syscallArgs_a9(R9), R1
	MOVD R1, (STACK_ARGS+3*8)(R15)
	MOVD syscallArgs_a10(R9), R1
	MOVD R1, (STACK_ARGS+4*8)(R15)
	MOVD syscallArgs_a11(R9), R1
	MOVD R1, (STACK_ARGS+5*8)(R15)
	MOVD syscallArgs_a12(R9), R1
	MOVD R1, (STACK_ARGS+6*8)(R15)
	MOVD syscallArgs_a13(R9), R1
	MOVD R1, (STACK_ARGS+7*8)(R15)
	MOVD syscallArgs_a14(R9), R1
	MOVD R1, (STACK_ARGS+8*8)(R15)
	MOVD syscallArgs_a15(R9), R1
	MOVD R1, (STACK_ARGS+9*8)(R15)
	MOVD syscallArgs_a16(R9), R1
	MOVD R1, (STACK_ARGS+10*8)(R15)
	MOVD syscallArgs_a17(R9), R1
	MOVD R1, (STACK_ARGS+11*8)(R15)
	MOVD syscallArgs_a18(R9), R1
	MOVD R1, (STACK_ARGS+12*8)(R15)
	MOVD syscallArgs_a19(R9), R1
	MOVD R1, (STACK_ARGS+13*8)(R15)
	MOVD syscallArgs_a20(R9), R1
	MOVD R1, (STACK_ARGS+14*8)(R15)
	MOVD syscallArgs_a21(R9), R1
	MOVD R1, (STACK_ARGS+15*8)(R15)
	MOVD syscallArgs_a22(R9), R1
	MOVD R1, (STACK_ARGS+16*8)(R15)
	MOVD syscallArgs_a23(R9), R1
	MOVD R1, (STACK_ARGS+17*8)(R15)
	MOVD syscallArgs_a24(R9), R1
	MOVD R1, (STACK_ARGS+18*8)(R15)
	MOVD syscallArgs_a25(R9), R1
	MOVD R1, (STACK_ARGS+19*8)(R15)
	MOVD syscallArgs_a26(R9), R1
	MOVD R1, (STACK_ARGS+20*8)(R15)
	MOVD syscallArgs_a27(R9), R1
	MOVD R1, (STACK_ARGS+21*8)(R15)
	MOVD syscallArgs_a28(R9), R1
	MOVD R1, (STACK_ARGS+22*8)(R15)
	MOVD syscallArgs_a29(R9), R1
	MOVD R1, (STACK_ARGS+23*8)(R15)
	MOVD syscallArgs_a30(R9), R1
	MOVD R1, (STACK_ARGS+24*8)(R15)
	MOVD syscallArgs_a31(R9), R1
	MOVD R1, (STACK_ARGS+25*8)(R15)
	MOVD syscallArgs_a32(R9), R1
	MOVD R1, (STACK_ARGS+26*8)(R15)

	// Call function
	MOVD syscallArgs_fn(R9), R1
	BL   (R1)

	// Restore args pointer for storing results
	MOVD ARGP_SAVE(R15), R9

	// Store integer results back (R2, R3)
	MOVD R2, syscallArgs_a1(R9)
	MOVD R3, syscallArgs_a2(R9)
	MOVD $0, syscallArgs_a3(R9) // no errno here: clear the input argument

	// Store float return values (F0, F2, F4, F6)
	FMOVD F0, syscallArgs_f1(R9)
	FMOVD F2, syscallArgs_f2(R9)
	FMOVD F4, syscallArgs_f3(R9)
	FMOVD F6, syscallArgs_f4(R9)

	// Deallocate stack frame
	ADD $STACK_SIZE, R15

	// Restore callee-saved registers from caller's save area
	LMG 48(R15), R6, R15

	RET
