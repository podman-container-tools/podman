// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

//go:build !cgo && linux

#include "textflag.h"
#include "go_asm.h"
#include "abi_loong64.h"

// these trampolines map the gcc ABI to Go ABI and then calls into the Go equivalent functions.
// R23 is used as temporary register.

TEXT x_cgo_init_trampoline(SB), NOSPLIT, $16
	MOVV R4, 8(R3)
	MOVV R5, 16(R3)
	MOVV ·x_cgo_init_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $8
	MOVV R4, 8(R3)
	MOVV ·x_cgo_thread_start_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $8
	MOVV R4, 8(R3)
	MOVV ·x_cgo_setenv_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $8
	MOVV R4, 8(R3)
	MOVV ·x_cgo_unsetenv_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_notify_runtime_init_done(SB)
	RET

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_bindm(SB)
	RET

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0
	MOVV G+8(FP), R4
	MOVV setg+0(FP), R23
	CALL (R23)
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $0
	// See crosscall2.
	ADDV $(-23*8), R3
	MOVV R4, (1*8)(R3) // fn unsafe.Pointer
	MOVV R5, (2*8)(R3) // a unsafe.Pointer
	MOVV R7, (3*8)(R3) // ctxt uintptr

	SAVE_R22_TO_R31((4*8))
	SAVE_F24_TO_F31((14*8))
	MOVV R1, (22*8)(R3)

	MOVV ·threadentry_call(SB), R23
	MOVV (R23), R23
	CALL (R23)

	RESTORE_R22_TO_R31((4*8))
	RESTORE_F24_TO_F31((14*8))
	MOVV (22*8)(R3), R1

	ADDV $(23*8), R3
	RET

TEXT ·call5(SB), NOSPLIT, $0-0
	MOVV fn+0(FP), R23
	MOVV a1+8(FP), R4
	MOVV a2+16(FP), R5
	MOVV a3+24(FP), R6
	MOVV a4+32(FP), R7
	MOVV a5+40(FP), R8
	CALL (R23)
	MOVV R4, ret+48(FP)
	RET
