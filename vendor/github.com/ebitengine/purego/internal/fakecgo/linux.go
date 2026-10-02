// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !cgo && linux

package fakecgo

import (
	"structs"
	"unsafe"
)

// argset matches runtime/cgocall.go:argset, which in turn matches
// runtime/cgo/linux_syscall.c:argset_t.
type argset struct {
	_      structs.HostLayout
	args   *uintptr
	retval uintptr
}

//go:nosplit
//go:norace
func (a *argset) arg(i int) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Add(unsafe.Pointer(a.args), uintptr(i)*unsafe.Sizeof(uintptr(0))))
}

//go:linkname _cgo_libc_setegid syscall.cgo_libc_setegid
//go:linkname _cgo_libc_seteuid syscall.cgo_libc_seteuid
//go:linkname _cgo_libc_setgid syscall.cgo_libc_setgid
//go:linkname _cgo_libc_setregid syscall.cgo_libc_setregid
//go:linkname _cgo_libc_setresgid syscall.cgo_libc_setresgid
//go:linkname _cgo_libc_setresuid syscall.cgo_libc_setresuid
//go:linkname _cgo_libc_setreuid syscall.cgo_libc_setreuid
//go:linkname _cgo_libc_setuid syscall.cgo_libc_setuid
//go:linkname _cgo_libc_setgroups syscall.cgo_libc_setgroups

var _cgo_libc_setegid = &_cgo_purego_setegid_trampoline
var _cgo_libc_seteuid = &_cgo_purego_seteuid_trampoline
var _cgo_libc_setgid = &_cgo_purego_setgid_trampoline
var _cgo_libc_setregid = &_cgo_purego_setregid_trampoline
var _cgo_libc_setresgid = &_cgo_purego_setresgid_trampoline
var _cgo_libc_setresuid = &_cgo_purego_setresuid_trampoline
var _cgo_libc_setreuid = &_cgo_purego_setreuid_trampoline
var _cgo_libc_setuid = &_cgo_purego_setuid_trampoline
var _cgo_libc_setgroups = &_cgo_purego_setgroups_trampoline

//go:nosplit
//go:norace
func errno() int32 {
	// this indirection is to avoid go vet complaining about possible misuse of unsafe.Pointer
	loc := __errno_location()
	return **(**int32)(unsafe.Pointer(&loc))
}

//go:linkname _cgo_purego_setegid_trampoline _cgo_purego_setegid_trampoline
var _cgo_purego_setegid_trampoline byte
var x_cgo_purego_setegid_call = x_cgo_purego_setegid

//go:nosplit
//go:norace
func x_cgo_purego_setegid(c *argset) {
	ret := setegid(uint32(uintptr(c.arg(0))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_seteuid_trampoline _cgo_purego_seteuid_trampoline
var _cgo_purego_seteuid_trampoline byte
var x_cgo_purego_seteuid_call = x_cgo_purego_seteuid

//go:nosplit
//go:norace
func x_cgo_purego_seteuid(c *argset) {
	ret := seteuid(uint32(uintptr(c.arg(0))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setgid_trampoline _cgo_purego_setgid_trampoline
var _cgo_purego_setgid_trampoline byte
var x_cgo_purego_setgid_call = x_cgo_purego_setgid

//go:nosplit
//go:norace
func x_cgo_purego_setgid(c *argset) {
	ret := setgid(uint32(uintptr(c.arg(0))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setregid_trampoline _cgo_purego_setregid_trampoline
var _cgo_purego_setregid_trampoline byte
var x_cgo_purego_setregid_call = x_cgo_purego_setregid

//go:nosplit
//go:norace
func x_cgo_purego_setregid(c *argset) {
	ret := setregid(uint32(uintptr(c.arg(0))), uint32(uintptr(c.arg(1))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setresgid_trampoline _cgo_purego_setresgid_trampoline
var _cgo_purego_setresgid_trampoline byte
var x_cgo_purego_setresgid_call = x_cgo_purego_setresgid

//go:nosplit
//go:norace
func x_cgo_purego_setresgid(c *argset) {
	ret := setresgid(uint32(uintptr(c.arg(0))), uint32(uintptr(c.arg(1))), uint32(uintptr(c.arg(2))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setresuid_trampoline _cgo_purego_setresuid_trampoline
var _cgo_purego_setresuid_trampoline byte
var x_cgo_purego_setresuid_call = x_cgo_purego_setresuid

//go:nosplit
//go:norace
func x_cgo_purego_setresuid(c *argset) {
	ret := setresuid(uint32(uintptr(c.arg(0))), uint32(uintptr(c.arg(1))), uint32(uintptr(c.arg(2))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setreuid_trampoline _cgo_purego_setreuid_trampoline
var _cgo_purego_setreuid_trampoline byte
var x_cgo_purego_setreuid_call = x_cgo_purego_setreuid

//go:nosplit
//go:norace
func x_cgo_purego_setreuid(c *argset) {
	ret := setreuid(uint32(uintptr(c.arg(0))), uint32(uintptr(c.arg(1))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setuid_trampoline _cgo_purego_setuid_trampoline
var _cgo_purego_setuid_trampoline byte
var x_cgo_purego_setuid_call = x_cgo_purego_setuid

//go:nosplit
//go:norace
func x_cgo_purego_setuid(c *argset) {
	ret := setuid(uint32(uintptr(c.arg(0))))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}

//go:linkname _cgo_purego_setgroups_trampoline _cgo_purego_setgroups_trampoline
var _cgo_purego_setgroups_trampoline byte
var x_cgo_purego_setgroups_call = x_cgo_purego_setgroups

//go:nosplit
//go:norace
func x_cgo_purego_setgroups(c *argset) {
	ret := setgroups(uint32(uintptr(c.arg(0))), (*uint32)(c.arg(1)))
	if ret == -1 {
		c.retval = uintptr(errno())
	} else {
		c.retval = uintptr(ret)
	}
}
