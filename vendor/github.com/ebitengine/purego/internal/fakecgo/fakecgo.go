// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

//go:build !cgo && (darwin || freebsd || linux || netbsd)

package fakecgo

import _ "unsafe"

// setg_trampoline calls setg with the G provided
func setg_trampoline(setg uintptr, G uintptr)

// call5 takes fn the C function and 5 arguments and calls the function with those arguments
func call5(fn, a1, a2, a3, a4, a5 uintptr) uintptr
