// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !cgo && (freebsd || linux)

#include "textflag.h"
#include "go_asm.h"

// These trampolines map the gcc ABI to Go ABI0 and then call into the Go equivalent functions.
// On ARM32, Go ABI0 uses stack-based calling convention.
// Arguments are placed on the stack starting at 4(SP) after the prologue.

TEXT x_cgo_init_trampoline(SB), NOSPLIT, $8-0
	MOVW R0, 4(R13)
	MOVW R1, 8(R13)
	MOVW ·x_cgo_init_call(SB), R12
	MOVW (R12), R12
	CALL (R12)
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $8-0
	MOVW R0, 4(R13)
	MOVW ·x_cgo_thread_start_call(SB), R12
	MOVW (R12), R12
	CALL (R12)
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $8-0
	MOVW R0, 4(R13)
	MOVW ·x_cgo_setenv_call(SB), R12
	MOVW (R12), R12
	CALL (R12)
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $8-0
	MOVW R0, 4(R13)
	MOVW ·x_cgo_unsetenv_call(SB), R12
	MOVW (R12), R12
	CALL (R12)
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0-0
	CALL ·x_cgo_notify_runtime_init_done(SB)
	RET

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_bindm(SB)
	RET

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0-8
	MOVW G+4(FP), R0
	MOVW setg+0(FP), R12
	BL   (R12)
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $104-0
	// Save C callee-saved registers at C-to-Go boundary.
	// See crosscall2 in asm_arm.s.
	// ARM AAPCS callee-saved: R4-R11 (includes g=R10), D8-D15.
	// LR is saved/restored by the Go-managed frame prologue/epilogue.
	MOVW R0, 4(R13) // arg for threadentry_call

	MOVW R4, 8(R13)
	MOVW R5, 12(R13)
	MOVW R6, 16(R13)
	MOVW R7, 20(R13)
	MOVW R8, 24(R13)
	MOVW R9, 28(R13)
	MOVW g, 32(R13)   // R10
	MOVW R11, 36(R13)

	// Skip floating point registers if runtime.goarmsoftfp!=0.
	MOVB    runtime·goarmsoftfp(SB), R11
	CMP     $0, R11
	BNE     skipfpsave
	MOVD F8, 40(R13)
	MOVD F9, 48(R13)
	MOVD F10, 56(R13)
	MOVD F11, 64(R13)
	MOVD F12, 72(R13)
	MOVD F13, 80(R13)
	MOVD F14, 88(R13)
	MOVD F15, 96(R13)

skipfpsave:
	MOVW ·threadentry_call(SB), R12
	MOVW (R12), R12
	CALL (R12)

	MOVB    runtime·goarmsoftfp(SB), R11
	CMP     $0, R11
	BNE     skipfprest
	MOVD 40(R13), F8
	MOVD 48(R13), F9
	MOVD 56(R13), F10
	MOVD 64(R13), F11
	MOVD 72(R13), F12
	MOVD 80(R13), F13
	MOVD 88(R13), F14
	MOVD 96(R13), F15

skipfprest:
	MOVW 8(R13), R4
	MOVW 12(R13), R5
	MOVW 16(R13), R6
	MOVW 20(R13), R7
	MOVW 24(R13), R8
	MOVW 28(R13), R9
	MOVW 32(R13), g
	MOVW 36(R13), R11

	RET

TEXT ·call5(SB), NOSPLIT, $8-28
	MOVW fn+0(FP), R12
	MOVW a1+4(FP), R0
	MOVW a2+8(FP), R1
	MOVW a3+12(FP), R2
	MOVW a4+16(FP), R3
	MOVW a5+20(FP), R4

	// Store 5th arg below SP (in local frame area)
	MOVW R4, arg5-8(SP)

	// Align SP to 8 bytes for call (required by ARM AAPCS)
	SUB  $8, R13
	CALL (R12)
	ADD  $8, R13
	MOVW R0, r1+24(FP)
	RET
