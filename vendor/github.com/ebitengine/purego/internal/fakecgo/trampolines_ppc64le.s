// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !cgo && linux

#include "textflag.h"
#include "go_asm.h"

// These trampolines map the C ABI to Go ABI and call into the Go equivalent functions.
//
// PPC64LE ELFv2 ABI stack frame layout:
//   0(R1)  = backchain (pointer to caller's frame)
//   8(R1)  = CR save area
//   16(R1) = LR save area
//   24(R1) = reserved
//   32(R1) = parameter save area (minimum 64 bytes for 8 args)
//
// Two patterns are used depending on call direction:
//
// C→Go trampolines: The C caller already provides a 32-byte linkage area.
//   Save LR/CR into caller's frame at 16(R1)/8(R1) BEFORE allocating,
//   then use MOVDU to allocate and set backchain atomically.
//
// Go→C trampolines: Go callers don't provide ELFv2 linkage area.
//   Allocate frame first with MOVDU, then save LR/CR into OUR frame.

TEXT x_cgo_init_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	// R3, R4 already have the arguments
	MOVD ·x_cgo_init_call(SB), R12
	MOVD (R12), R12
	MOVD R12, CTR
	CALL CTR

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	MOVD ·x_cgo_thread_start_call(SB), R12
	MOVD (R12), R12
	MOVD R12, CTR
	CALL CTR

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

// void (*_cgo_setenv)(char**)
// C arg: R3 = pointer to env
// This is C→Go: caller is C ABI.
TEXT x_cgo_setenv_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	MOVD ·x_cgo_setenv_call(SB), R12
	MOVD (R12), R12
	MOVD R12, CTR
	CALL CTR

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	MOVD ·x_cgo_unsetenv_call(SB), R12
	MOVD (R12), R12
	MOVD R12, CTR
	CALL CTR

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	CALL ·x_cgo_notify_runtime_init_done(SB)

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	CALL ·x_cgo_bindm(SB)

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

TEXT ·setg_trampoline(SB), NOSPLIT|NOFRAME, $0-16
	// Save LR, CR, and R31 to non-volatile registers (C ABI preserves R14-R31)
	MOVD LR, R20
	MOVW CR, R21
	MOVD R31, R22 // save R31 because load_g clobbers it

	// Load arguments from Go stack
	MOVD 32(R1), R12 // setg function pointer
	MOVD 40(R1), R3  // g pointer → first C arg

	// Allocate ELFv2 frame for the C callee (32 bytes minimum)
	MOVDU R1, -32(R1)

	// Call setg_gcc which stores g to TLS
	MOVD R12, CTR
	CALL CTR

	// setg_gcc stored g to TLS but restored old g in R30.
	// Call load_g to reload g from TLS into R30.
	// Note: load_g clobbers R31
	CALL runtime·load_g(SB)

	// Deallocate frame
	ADD $32, R1

	// Clear R0 before returning to Go code.
	// Go uses R0 as a constant 0 for things like "std r0,X(r1)" to zero stack locations.
	// C/assembly functions may leave garbage in R0.
	XOR R0, R0, R0

	// Restore LR, CR, and R31 from non-volatile registers
	MOVD R22, R31 // restore R31
	MOVD R20, LR
	MOVW R21, CR
	RET

TEXT threadentry_trampoline(SB), NOSPLIT|NOFRAME, $0-0
	MOVD LR, 16(R1)
	MOVW CR, R0
	MOVD R0, 8(R1)

	MOVDU R1, -32(R1)

	MOVD ·threadentry_call(SB), R12
	MOVD (R12), R12
	MOVD R12, CTR
	CALL CTR

	ADD $32, R1

	MOVD 16(R1), LR
	MOVD 8(R1), R0
	MOVW R0, CR
	RET

TEXT ·call5(SB), NOSPLIT|NOFRAME, $0-56
	MOVD LR, R20
	MOVW CR, R21

	// Load arguments from Go stack into C argument registers
	// Go placed args at 32(R1), 40(R1), etc.
	MOVD 32(R1), R12 // fn
	MOVD 40(R1), R3  // a1 → first C arg
	MOVD 48(R1), R4  // a2 → second C arg
	MOVD 56(R1), R5  // a3 → third C arg
	MOVD 64(R1), R6  // a4 → fourth C arg
	MOVD 72(R1), R7  // a5 → fifth C arg

	MOVDU R1, -32(R1)

	MOVD R12, CTR
	CALL CTR

	// Store return value
	// After MOVDU -32, original 80(R1) is now at 80+32=112(R1)
	MOVD R3, (80+32)(R1)

	// Deallocate frame
	ADD $32, R1

	// Clear R0 before returning to Go code.
	// Go uses R0 as a constant 0 register for things like "std r0,X(r1)"
	// to zero stack locations. C functions may leave garbage in R0.
	XOR R0, R0, R0

	// Restore LR/CR from non-volatile registers
	MOVD R20, LR
	MOVW R21, CR
	RET
