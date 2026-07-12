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

TEXT threadentry_trampoline(SB), NOSPLIT, $8-0
	// See crosscall2.
	MOVW R0, 4(R13)
	MOVW ·threadentry_call(SB), R12
	MOVW (R12), R12
	CALL (R12)
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
