// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

TEXT callbackasm1(SB), NOFRAME, $0
	NO_LOCAL_POINTERS

	// On entry, the trampoline in zcallback_riscv64.s left
	// the callback index in X7.

	// Save callback register arguments X10-X17 and F10-F17.
	// Stack args (if any) are at 0(SP), 8(SP), etc.
	// We save register args at SP-128, making them contiguous with stack args.
	ADD $-(16*8), SP, X6

	// Save float arg regs fa0-fa7 (F10-F17)
	MOVD F10, 0(X6)
	MOVD F11, 8(X6)
	MOVD F12, 16(X6)
	MOVD F13, 24(X6)
	MOVD F14, 32(X6)
	MOVD F15, 40(X6)
	MOVD F16, 48(X6)
	MOVD F17, 56(X6)

	// Save integer arg regs a0-a7 (X10-X17)
	MOV X10, 64(X6)
	MOV X11, 72(X6)
	MOV X12, 80(X6)
	MOV X13, 88(X6)
	MOV X14, 96(X6)
	MOV X15, 104(X6)
	MOV X16, 112(X6)
	MOV X17, 120(X6)

	// Allocate space on stack for RA, saved regs, and callbackArgs.
	// We need: 8 (RA) + 8 (X9 callee-saved) + 24 (callbackArgs) = 40, round to 176 (22*8)
	// to match loong64 and ensure we don't overlap with saved register args.
	// The saved regs end at SP-8 (original), so we need new SP below SP-128.
	ADD $-(22*8), SP

	// Save link register (RA/X1) and callee-saved register X9
	// (X9 is used by the assembler for some instructions)
	MOV X1, 0(SP)
	MOV X9, 8(SP)

	// Create a struct callbackArgs on our stack.
	// callbackArgs struct: index(0), args(8), result(16)
	// Place it at 16(SP) to avoid overlap
	ADD $16, SP, X9
	MOV X7, 0(X9)   // callback index
	MOV X6, 8(X9)   // address of args vector
	MOV X0, 16(X9)  // result = 0

	// Call crosscall2 with arguments in registers
	MOV ·callbackWrap_call(SB), X10 // Get the ABIInternal function pointer
	MOV (X10), X10                  // without <ABIInternal> by using a closure.  X10 = fn
	MOV X9, X11                     // X11 = frame (address of callbackArgs)
	MOV X0, X13                     // X13 = ctxt = 0

	CALL crosscall2(SB)

	// Get callback result.
	ADD $16, SP, X9
	MOV 16(X9), X10

	// Restore link register and callee-saved X9
	MOV 8(SP), X9
	MOV 0(SP), X1

	// Restore stack pointer
	ADD $(22*8), SP

	RET
