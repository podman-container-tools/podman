// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

package purego

import (
	"reflect"
	"syscall"
	"unsafe"
)

var syscallXABI0 uintptr

func syscall_syscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	r1, r2, errno := syscall.SyscallN(fn, args...)
	return r1, r2, uintptr(errno)
}

// NewCallback converts a Go function to a function pointer conforming to the stdcall calling convention.
// This is useful when interoperating with Windows code requiring callbacks. The argument is expected to be a
// function with one uintptr-sized result. The function must not have arguments with size larger than the
// size of uintptr. Only a limited number of callbacks may be created in a single Go process, and any memory
// allocated for these callbacks is never released. Between NewCallback and NewCallbackCDecl, at least 1024
// callbacks can always be created. Although this function is similar to the darwin version it may act
// differently.
func NewCallback(fn any) uintptr {
	isCDecl := false
	ty := reflect.TypeOf(fn)
	for i := range ty.NumIn() {
		in := ty.In(i)
		if !in.AssignableTo(reflect.TypeFor[CDecl]()) {
			continue
		}
		if i != 0 {
			panic("purego: CDecl must be the first argument")
		}
		isCDecl = true
	}
	if isCDecl {
		return syscall.NewCallbackCDecl(fn)
	}
	return syscall.NewCallback(fn)
}

func loadSymbol(handle uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}

// callbackMaxFrame is a stub definition.
const callbackMaxFrame = 0

func callbackArgFromStack(argsBase unsafe.Pointer, stackSlot int, stackByteOffset *uintptr, inType reflect.Type) reflect.Value {
	panic("purego: callbackArgFromStack should not be called on windows")
}
