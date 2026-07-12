// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build !cgo && (darwin || freebsd || linux)

#include "textflag.h"
#include "go_asm.h"
#include "abi_arm64.h"

// These trampolines map the gcc ABI to Go ABIInternal and then calls into the Go equivalent functions.
// Note that C arguments are passed in R0-R7, which matches Go ABIInternal for the first eight arguments.
// R9 is used as a temporary register.

TEXT x_cgo_init_trampoline(SB), NOSPLIT, $0-0
	MOVD ·x_cgo_init_call(SB), R9
	MOVD (R9), R9
	CALL R9
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $0-0
	MOVD ·x_cgo_thread_start_call(SB), R9
	MOVD (R9), R9
	CALL R9
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $0-0
	MOVD ·x_cgo_setenv_call(SB), R9
	MOVD (R9), R9
	CALL R9
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $0-0
	MOVD ·x_cgo_unsetenv_call(SB), R9
	MOVD (R9), R9
	CALL R9
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0-0
	CALL ·x_cgo_notify_runtime_init_done(SB)
	RET

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_bindm(SB)
	RET

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0-16
	MOVD G+8(FP), R0
	MOVD setg+0(FP), R9
	CALL R9
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $0-0
	// See crosscall2.
	SUB  $(8*24), RSP
	STP  (R0, R1), (8*1)(RSP)
	MOVD R3, (8*3)(RSP)

	SAVE_R19_TO_R28(8*4)
	SAVE_F8_TO_F15(8*14)
	STP (R29, R30), (8*22)(RSP)

	MOVD ·threadentry_call(SB), R9
	MOVD (R9), R9
	CALL R9
	MOVD $0, R0                    // TODO: get the return value from threadentry

	RESTORE_R19_TO_R28(8*4)
	RESTORE_F8_TO_F15(8*14)
	LDP (8*22)(RSP), (R29, R30)

	ADD $(8*24), RSP
	RET

TEXT ·call5(SB), NOSPLIT, $0-0
	MOVD fn+0(FP), R9
	MOVD a1+8(FP), R0
	MOVD a2+16(FP), R1
	MOVD a3+24(FP), R2
	MOVD a4+32(FP), R3
	MOVD a5+40(FP), R4
	CALL R9
	MOVD R0, ret+48(FP)
	RET
