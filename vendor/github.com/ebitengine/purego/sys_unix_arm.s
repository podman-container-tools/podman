// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

TEXT callbackasm1(SB), NOSPLIT|NOFRAME, $0
	NO_LOCAL_POINTERS

	// Allocate stack frame: 48 + 16 + 64 + 16 = 144 bytes
	SUB $144, R13

	// Save callee-saved registers at SP+0
	MOVW R4, 0(R13)
	MOVW R5, 4(R13)
	MOVW R6, 8(R13)
	MOVW R7, 12(R13)
	MOVW R8, 16(R13)
	MOVW R9, 20(R13)
	MOVW g, 24(R13)
	MOVW R11, 28(R13)
	MOVW R14, 32(R13)

	// Save callback index (passed in R12) at SP+36
	MOVW R12, 36(R13)

	// Save integer arguments R0-R3 at SP+128 (frame[16..19])
	MOVW R0, 128(R13)
	MOVW R1, 132(R13)
	MOVW R2, 136(R13)
	MOVW R3, 140(R13)

	// Save floating point registers F0-F7 at SP+64 (frame[0..15])
	// Note: We always save these since we target hard-float ABI.
	MOVD F0, 64(R13)
	MOVD F1, 72(R13)
	MOVD F2, 80(R13)
	MOVD F3, 88(R13)
	MOVD F4, 96(R13)
	MOVD F5, 104(R13)
	MOVD F6, 112(R13)
	MOVD F7, 120(R13)

	// Set up callbackArgs at SP+48
	MOVW 36(R13), R4
	MOVW R4, 48(R13)
	ADD  $64, R13, R4
	MOVW R4, 52(R13)
	MOVW $0, R4
	MOVW R4, 56(R13)

	// Call crosscall2(fn, frame, 0, ctxt)
	MOVW ·callbackWrap_call(SB), R0
	MOVW (R0), R0
	ADD  $48, R13, R1
	MOVW $0, R2
	MOVW $0, R3

	BL crosscall2(SB)

	// Get result
	MOVW 56(R13), R0

	// Restore float registers
	MOVD 64(R13), F0
	MOVD 72(R13), F1
	MOVD 80(R13), F2
	MOVD 88(R13), F3
	MOVD 96(R13), F4
	MOVD 104(R13), F5
	MOVD 112(R13), F6
	MOVD 120(R13), F7

	// Restore callee-saved registers
	MOVW 0(R13), R4
	MOVW 4(R13), R5
	MOVW 8(R13), R6
	MOVW 12(R13), R7
	MOVW 16(R13), R8
	MOVW 20(R13), R9
	MOVW 24(R13), g
	MOVW 28(R13), R11
	MOVW 32(R13), R14

	ADD $144, R13
	RET
