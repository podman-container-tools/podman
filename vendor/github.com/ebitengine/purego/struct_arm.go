// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

package purego

import (
	"reflect"
	"unsafe"
)

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []any) []any {
	size := v.Type().Size()
	if size == 0 {
		return keepAlive
	}

	// TODO: ARM EABI: small structs are passed in registers or on stack
	// For simplicity, pass by pointer for now
	ptr := v.Addr().UnsafePointer()
	keepAlive = append(keepAlive, ptr)
	if *numInts < 4 {
		addInt(uintptr(ptr))
		*numInts++
	} else {
		addStack(uintptr(ptr))
		*numStack++
	}
	return keepAlive
}

func getStruct(outType reflect.Type, syscall syscall15Args) (v reflect.Value) {
	outSize := outType.Size()
	if outSize == 0 {
		return reflect.New(outType).Elem()
	}
	if outSize <= 4 {
		// Fits in one register
		return reflect.NewAt(outType, unsafe.Pointer(&struct{ a uintptr }{syscall.a1})).Elem()
	}
	if outSize <= 8 {
		// Fits in two registers
		return reflect.NewAt(outType, unsafe.Pointer(&struct{ a, b uintptr }{syscall.a1, syscall.a2})).Elem()
	}
	// Larger structs returned via pointer in a1
	return reflect.NewAt(outType, *(*unsafe.Pointer)(unsafe.Pointer(&syscall.a1))).Elem()
}

func placeRegisters(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) {
	// TODO: For ARM32, just pass the struct data directly
	// This is a simplified implementation
	size := v.Type().Size()
	if size == 0 {
		return
	}
	ptr := unsafe.Pointer(v.UnsafeAddr())
	if size <= 4 {
		addInt(*(*uintptr)(ptr))
	} else if size <= 8 {
		addInt(*(*uintptr)(ptr))
		addInt(*(*uintptr)(unsafe.Add(ptr, 4)))
	}
}

// shouldBundleStackArgs always returns false on arm
// since C-style stack argument bundling is only needed on Darwin ARM64.
func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	return false
}

// structFitsInRegisters is not used on arm.
func structFitsInRegisters(val reflect.Value, tempNumInts, tempNumFloats int) (bool, int, int) {
	panic("purego: structFitsInRegisters should not be called on arm")
}

// collectStackArgs is not used on arm.
func collectStackArgs(args []reflect.Value, startIdx int, numInts, numFloats int,
	keepAlive []any, addInt, addFloat, addStack func(uintptr),
	pNumInts, pNumFloats, pNumStack *int) ([]reflect.Value, []any) {
	panic("purego: collectStackArgs should not be called on arm")
}

// bundleStackArgs is not used on arm.
func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	panic("purego: bundleStackArgs should not be called on arm")
}
