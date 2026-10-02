// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !cgo && !amd64 && (darwin || freebsd || linux || netbsd)

package fakecgo

import "unsafe"

// callThreadEntryFn calls fn (runtime.mstart). On architectures without the
// amd64 frame-pointer epilogue issue this is a plain indirect call. It must be
// nosplit and norace like the threadentry callers so it neither inserts a
// morestack preamble nor runs race instrumentation during the fragile
// thread-bootstrap window.
//
//go:nosplit
//go:norace
func callThreadEntryFn(fn uintptr) {
	// fn is the code pointer. Build a func value whose first word is fn by
	// pointing the closure at &fn, then call it (same trick fakecgo has always
	// used to call a raw PC from Go).
	fnPtr := uintptr(unsafe.Pointer(&fn))
	(*(*func())(unsafe.Pointer(&fnPtr)))()
}
