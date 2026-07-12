// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

// S390X ELF ABI callbackasm1 implementation
// On entry, R0 contains the callback index (set by callbackasm)
// NOTE: We use R0 instead of R11 because R11 is callee-saved on S390X.
//
// S390X stack frame layout:
//   0(R15)   - back chain
//  48(R15)   - register save area (R6-R15)
// 160(R15)  - parameter area
//
// S390X uses R2-R6 for integer arguments (5 registers) and F0,F2,F4,F6 for floats (4 registers).
//
// Our frame layout (total 264 bytes, 8-byte aligned):
//  0(R15)    - back chain
// 48(R15)   - saved R6-R15 (done by STMG)
// 160(R15)  - callbackArgs struct (32 bytes: index, args, result, stackArgs)
// 192(R15)  - args array start
//
// Args array layout:
// - floats F0,F2,F4,F6 (32 bytes)
// - ints R2-R6 (40 bytes)
// Total args array: 72 bytes, ends at 264
//
// Stack args in caller's frame start at old_R15+160

#define FRAME_SIZE     264
#define CB_ARGS        160
#define ARGS_ARRAY     192
#define FLOAT_OFF      0
#define INT_OFF        32

TEXT callbackasm1(SB), NOSPLIT|NOFRAME, $0
	NO_LOCAL_POINTERS

	// On entry, the trampoline in zcallback_s390x.s left
	// the callback index in R0 (NOT R11, since R11 is callee-saved).
	// R6 contains the 5th integer argument.

	// Save R6-R15 in caller's frame (per S390X ABI) BEFORE allocating our frame
	// STMG stores R6's current value (the 5th arg) at 48(R15)
	STMG R6, R15, 48(R15)

	// Save current stack pointer (will be back chain)
	MOVD R15, R1

	// Allocate our stack frame
	SUB  $FRAME_SIZE, R15
	MOVD R1, 0(R15)       // back chain

	// Save R0 (callback index) immediately - it's volatile
	MOVD R0, (CB_ARGS+0)(R15)

	// Save callback arguments to args array.
	// Layout: floats first (F0,F2,F4,F6), then ints (R2-R6)
	FMOVD F0, (ARGS_ARRAY+FLOAT_OFF+0*8)(R15)
	FMOVD F2, (ARGS_ARRAY+FLOAT_OFF+1*8)(R15)
	FMOVD F4, (ARGS_ARRAY+FLOAT_OFF+2*8)(R15)
	FMOVD F6, (ARGS_ARRAY+FLOAT_OFF+3*8)(R15)

	MOVD R2, (ARGS_ARRAY+INT_OFF+0*8)(R15)
	MOVD R3, (ARGS_ARRAY+INT_OFF+1*8)(R15)
	MOVD R4, (ARGS_ARRAY+INT_OFF+2*8)(R15)
	MOVD R5, (ARGS_ARRAY+INT_OFF+3*8)(R15)

	// R6 (5th int arg) was saved at 48(old_R15) by STMG
	// old_R15 = current R15 + FRAME_SIZE, so R6 is at 48+FRAME_SIZE(R15) = 312(R15)
	MOVD (48+FRAME_SIZE)(R15), R1
	MOVD R1, (ARGS_ARRAY+INT_OFF+4*8)(R15)

	// Finish setting up callbackArgs struct at CB_ARGS(R15)
	// struct { index uintptr; args unsafe.Pointer; result uintptr; stackArgs unsafe.Pointer }
	// Note: index was already saved earlier
	ADD  $ARGS_ARRAY, R15, R1
	MOVD R1, (CB_ARGS+8)(R15)  // args = address of register args
	MOVD $0, (CB_ARGS+16)(R15) // result = 0

	// stackArgs points to caller's stack arguments at old_R15+160 = R15+FRAME_SIZE+160
	ADD  $(FRAME_SIZE+160), R15, R1
	MOVD R1, (CB_ARGS+24)(R15)      // stackArgs = &caller_stack_args

	// Call crosscall2 with arguments in registers:
	// R2 = fn (from callbackWrap_call closure)
	// R3 = frame (address of callbackArgs)
	// R5 = ctxt (0)
	MOVD ·callbackWrap_call(SB), R2
	MOVD (R2), R2                   // dereference closure to get fn
	ADD  $CB_ARGS, R15, R3          // frame = &callbackArgs
	MOVD $0, R5                     // ctxt = 0

	BL crosscall2(SB)

	// Get callback result into R2
	MOVD (CB_ARGS+16)(R15), R2

	// Deallocate frame
	ADD $FRAME_SIZE, R15

	// Restore R6-R15 from caller's frame
	LMG 48(R15), R6, R15

	RET
