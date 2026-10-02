// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !cgo && linux

#include "textflag.h"

TEXT _cgo_purego_setegid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setegid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_seteuid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_seteuid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setgid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setgid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setregid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setregid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setresgid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setresgid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setresuid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setresuid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setreuid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setreuid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setuid_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setuid_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET

TEXT _cgo_purego_setgroups_trampoline(SB), NOSPLIT, $8
	MOVV ·x_cgo_purego_setgroups_call(SB), R23
	MOVV (R23), R23
	CALL (R23)
	RET
