// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !(amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)

package purego

import (
	"reflect"
	"unsafe"
)

// This file provides the struct-handling helpers for architectures that do
// not support struct arguments or returns: 386, and any architecture reachable
// only through the integer-only cgo fallback. Every struct operation panics.

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []any) []any {
	panic("purego: struct arguments are not supported on this architecture")
}

func getStruct(outType reflect.Type, syscall syscallArgs) reflect.Value {
	panic("purego: struct returns are not supported on this architecture")
}

// structReturnInMemory reports whether a struct return value is returned through
// a caller-allocated hidden pointer. Structs are unsupported on this
// architecture, so it always reports false.
func structReturnInMemory(reflect.Type) bool {
	return false
}

// shouldBundleStackArgs always returns false because C-style stack argument
// bundling is only needed on Darwin ARM64.
func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	return false
}

func collectStackArgs(args []reflect.Value, startIdx int, numInts, numFloats int,
	keepAlive []any, addInt, addFloat, addStack func(uintptr),
	pNumInts, pNumFloats, pNumStack *int) ([]reflect.Value, []any) {
	panic("purego: collectStackArgs should not be called on this architecture")
}

func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	panic("purego: bundleStackArgs should not be called on this architecture")
}

func getCallbackStruct(inType reflect.Type, frame unsafe.Pointer, floatsN *int, intsN *int, stackSlot *int, stackByteOffset *uintptr) reflect.Value {
	panic("purego: struct callback arguments are not supported on this architecture")
}

func setStruct(a *callbackArgs, ret reflect.Value) {
	panic("purego: struct returns are not supported on this architecture")
}
