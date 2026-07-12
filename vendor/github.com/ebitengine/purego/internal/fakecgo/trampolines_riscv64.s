// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !cgo && linux

#include "textflag.h"
#include "go_asm.h"

// these trampolines map the gcc ABI to Go ABI and then calls into the Go equivalent functions.
// X5 is used as temporary register.

TEXT x_cgo_init_trampoline(SB), NOSPLIT, $16
	MOV  X10, 8(SP)
	MOV  X11, 16(SP)
	MOV  ·x_cgo_init_call(SB), X5
	MOV  (X5), X5
	CALL X5
	RET

TEXT x_cgo_thread_start_trampoline(SB), NOSPLIT, $8
	MOV  X10, 8(SP)
	MOV  ·x_cgo_thread_start_call(SB), X5
	MOV  (X5), X5
	CALL X5
	RET

TEXT x_cgo_setenv_trampoline(SB), NOSPLIT, $8
	MOV  X10, 8(SP)
	MOV  ·x_cgo_setenv_call(SB), X5
	MOV  (X5), X5
	CALL X5
	RET

TEXT x_cgo_unsetenv_trampoline(SB), NOSPLIT, $8
	MOV  X10, 8(SP)
	MOV  ·x_cgo_unsetenv_call(SB), X5
	MOV  (X5), X5
	CALL X5
	RET

TEXT x_cgo_notify_runtime_init_done_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_notify_runtime_init_done(SB)
	RET

TEXT x_cgo_bindm_trampoline(SB), NOSPLIT, $0
	CALL ·x_cgo_bindm(SB)
	RET

// func setg_trampoline(setg uintptr, g uintptr)
TEXT ·setg_trampoline(SB), NOSPLIT, $0
	MOV  gp+8(FP), X10
	MOV  setg+0(FP), X5
	CALL X5
	RET

TEXT threadentry_trampoline(SB), NOSPLIT, $16
	MOV  X10, 8(SP)
	MOV  ·threadentry_call(SB), X5
	MOV  (X5), X5
	CALL X5
	RET

TEXT ·call5(SB), NOSPLIT, $0-48
	MOV  fn+0(FP), X5
	MOV  a1+8(FP), X10
	MOV  a2+16(FP), X11
	MOV  a3+24(FP), X12
	MOV  a4+32(FP), X13
	MOV  a5+40(FP), X14
	CALL X5
	MOV  X10, ret+48(FP)
	RET
