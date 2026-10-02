// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !cgo && (amd64 || arm64)

package fakecgo

import "unsafe"

//go:nosplit
func _cgo_sys_thread_start(ts *ThreadStart) {
	var attr pthread_attr_t
	var ign, oset sigset_t
	var p pthread_t
	var size size_t
	var err int

	// fprintf(stderr, "runtime/cgo: _cgo_sys_thread_start: fn=%p, g=%p\n", ts->fn, ts->g); // debug
	sigfillset(&ign)
	pthread_sigmask(SIG_SETMASK, &ign, &oset)

	pthread_attr_init(&attr)
	pthread_attr_getstacksize(&attr, &size)
	// Leave stacklo=0 and set stackhi=size; mstart will do the rest.
	ts.g.stackhi = uintptr(size)

	err = _cgo_try_pthread_create(&p, &attr, unsafe.Pointer(threadentry_trampolineABI0), ts)

	pthread_sigmask(SIG_SETMASK, &oset, nil)

	if err != 0 {
		print("fakecgo: pthread_create failed: ")
		println(err)
		abort()
	}
}

// threadentry_trampolineABI0 maps the C ABI to Go ABI then calls the Go function
//
//go:linkname x_threadentry_trampoline threadentry_trampoline
var x_threadentry_trampoline byte
var threadentry_trampolineABI0 = &x_threadentry_trampoline

//go:nosplit
func threadentry(v unsafe.Pointer) unsafe.Pointer {
	ts := *(*ThreadStart)(v)
	free(v)

	setg_trampoline(setg_func, uintptr(unsafe.Pointer(ts.g)))

	callThreadEntryFn(ts.fn)

	return nil
}

// here we will store a pointer to the provided setg func
var setg_func uintptr

// x_cgo_init(G *g, void (*setg)(void*)) (runtime/cgo/gcc_linux_amd64.c)
// This get's called during startup, adjusts stacklo, and provides a pointer to setg_gcc for us
// Additionally, if we set _cgo_init to non-null, go won't do it's own TLS setup
// This function can't be go:systemstack since go is not in a state where the systemcheck would work.
//
//go:nosplit
func x_cgo_init(g *G, setg uintptr) {
	var size size_t
	var attr pthread_attr_t

	setg_func = setg
	pthread_attr_init(&attr)
	pthread_attr_getstacksize(&attr, &size)
	// runtime/cgo uses __builtin_frame_address(0) instead of `uintptr(unsafe.Pointer(&size))`
	// but this should be OK since we are taking the address of the first variable in this function.
	g.stacklo = uintptr(unsafe.Pointer(&size)) - uintptr(size) + 4096
	pthread_attr_destroy(&attr)
}
