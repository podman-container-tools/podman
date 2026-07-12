// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build !cgo && (darwin || linux || freebsd)

/*
trampoline for emulating required C functions for cgo in go (see cgo.go)
(we convert cdecl calling convention to go and vice-versa)

C Calling convention cdecl used here (we only need integer args):
1. arg: DI
2. arg: SI
3. arg: DX
4. arg: CX
5. arg: R8
6. arg: R9
We don't need floats with these functions -> AX=0
return value will be in AX
temporary register is R11
*/
#include "textflag.h"
#include "go_asm.h"
#include "abi_amd64.h"

// these trampolines map the gcc ABI to Go ABI and then calls into the Go equivalent functions.

TEXT x_cgo_init_trampoline(SB), NOSPLIT, $16
	MOVQ DI, AX
	MOVQ SI, BX
	MOVQ ·x_cgo_init_call(SB), R11
	MOVQ (R11), R11
	CALL R11
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $8
	MOVQ DI, AX
	MOVQ ·x_cgo_thread_start_call(SB), R11
	MOVQ (R11), R11
	CALL R11
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $8
	MOVQ DI, AX
	MOVQ ·x_cgo_setenv_call(SB), R11
	MOVQ (R11), R11
	CALL R11
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $8
	MOVQ DI, AX
	MOVQ ·x_cgo_unsetenv_call(SB), R11
	MOVQ (R11), R11
	CALL R11
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0
	JMP ·x_cgo_notify_runtime_init_done(SB)

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT, $0
	JMP ·x_cgo_bindm(SB)

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0-16
	MOVQ G+8(FP), DI
	MOVQ setg+0(FP), R11
	XORL AX, AX
	CALL R11
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $0
	// See crosscall2.
	PUSH_REGS_HOST_TO_ABI0()

	// X15 is designated by Go as a fixed zero register.
	// Calling directly into ABIInternal, ensure it is zero.
	PXOR X15, X15

	MOVQ DI, AX
	MOVQ ·threadentry_call(SB), R11
	MOVQ (R11), R11
	CALL R11

	POP_REGS_HOST_TO_ABI0()
	RET

TEXT ·call5(SB), NOSPLIT, $0-56
	MOVQ fn+0(FP), R11
	MOVQ a1+8(FP), DI
	MOVQ a2+16(FP), SI
	MOVQ a3+24(FP), DX
	MOVQ a4+32(FP), CX
	MOVQ a5+40(FP), R8

	XORL AX, AX // no floats

	PUSHQ BP       // save BP
	MOVQ  SP, BP   // save SP inside BP bc BP is callee-saved
	SUBQ  $16, SP  // allocate space for alignment
	ANDQ  $-16, SP // align on 16 bytes for SSE

	CALL R11

	MOVQ BP, SP // get SP back
	POPQ BP     // restore BP

	MOVQ AX, ret+48(FP)
	RET
