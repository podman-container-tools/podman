// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build go1.27 && !cgo && linux

#include "textflag.h"
#include "go_asm.h"

// these trampolines map the gcc ABI to Go ABI and then calls into the Go equivalent functions.
// Note that C arguments are passed in R2-R6, which matches Go ABIInternal for the first five arguments.
// R1 is used as a temporary register.

TEXT x_cgo_init_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD R15, R1
	SUB  $192, R15
	MOVD R1, 0(R15)    // backchain
	MOVD R14, 160(R15) // save R14
	MOVD R9, 168(R15)  // save R9 (Go runtime needs this preserved)

	MOVD ·x_cgo_init_call(SB), R1
	MOVD (R1), R1
	BL   R1

	MOVD 168(R15), R9
	MOVD 160(R15), R14
	ADD  $192, R15
	BR   R14

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD R15, R1
	SUB  $176, R15
	MOVD R1, 0(R15)    // backchain
	MOVD R14, 152(R15) // save R14

	MOVD ·x_cgo_thread_start_call(SB), R1
	MOVD (R1), R1
	BL   R1

	MOVD 152(R15), R14
	ADD  $176, R15
	BR   R14

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD R15, R1
	SUB  $176, R15
	MOVD R1, 0(R15)    // backchain
	MOVD R14, 152(R15) // save R14

	MOVD ·x_cgo_setenv_call(SB), R1
	MOVD (R1), R1
	BL   R1

	MOVD 152(R15), R14
	ADD  $176, R15
	BR   R14

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD R15, R1
	SUB  $176, R15
	MOVD R1, 0(R15)    // backchain
	MOVD R14, 152(R15) // save R14

	MOVD ·x_cgo_unsetenv_call(SB), R1
	MOVD (R1), R1
	BL   R1

	MOVD 152(R15), R14
	ADD  $176, R15
	BR   R14

// These just tail-call into Go functions
TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	BR ·x_cgo_notify_runtime_init_done(SB)

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	BR ·x_cgo_bindm(SB)

// setg_trampoline(setg uintptr, g uintptr) - called from Go
TEXT ·setg_trampoline(SB), NOSPLIT|NOFRAME, $0-16
	MOVD 8(R15), R1  // setg function pointer
	MOVD 16(R15), R2 // g pointer -> C arg

	MOVD R14, R0
	MOVD R15, R3
	SUB  $160, R15
	MOVD R3, 0(R15)
	MOVD R0, 112(R15)
	MOVD R2, 120(R15)  // save newg before call

	BL R1 // call setg_gcc

	// Assign g directly instead of calling runtime·load_g
	// setg_gcc has already stored newg into TLS; put it in the g register too.
	MOVD 120(R15), g

	MOVD 112(R15), R14
	ADD  $160, R15
	BR   R14

TEXT threadentry_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	STMG R6, R15, 48(R15) // C save area
	MOVD R15, R1
	SUB  $176, R15
	MOVD R1, 0(R15)       // backchain

	MOVD ·threadentry_call(SB), R1
	MOVD (R1), R1
	BL   R1

	ADD $176, R15
	LMG 48(R15), R6, R15
	RET

TEXT ·call5(SB), NOSPLIT|NOFRAME, $0-56
	// Load Go args before modifying R15
	MOVD 8(R15), R1   // fn
	MOVD 16(R15), R7  // a1
	MOVD 24(R15), R8  // a2
	MOVD 32(R15), R9  // a3
	MOVD 40(R15), R10 // a4
	MOVD 48(R15), R11 // a5

	// Save state
	MOVD R15, R0    // original R15
	MOVD R12, R6    // Go's R12
	ADD  $-128, R15

	// Set up C frame with backchain
	MOVD R0, 0(R15) // backchain -> original R15
	MOVD R0, R3     // R3 = original R15 (can't use R0 as base!)
	MOVD 0(R3), R7  // save 0(original R15)
	MOVD $0, 0(R3)  // terminate backchain

	// Save context
	MOVD R14, 8(R15)
	MOVD R6, 16(R15) // R12
	MOVD R0, 24(R15) // original R15
	MOVD R7, 32(R15) // saved backchain

	// Set up C args (reload a1 since R7 was clobbered)
	MOVD 16(R3), R2 // a1 (use R3 as base, not R0!)
	MOVD R8, R3     // a2
	MOVD R9, R4     // a3
	MOVD R10, R5    // a4
	MOVD R11, R6    // a5

	BL R1

	// Store result and restore
	MOVD 24(R15), R3 // original R15
	MOVD R2, 56(R3)  // return value
	MOVD 32(R15), R7
	MOVD R7, 0(R3)   // restore backchain

	MOVD 8(R15), R14
	MOVD 16(R15), R12
	MOVD 24(R15), R15
	BR   R14
