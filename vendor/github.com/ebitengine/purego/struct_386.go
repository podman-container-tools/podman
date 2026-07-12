// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2025 The Ebitengine Authors

package purego

import "reflect"

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []any) []any {
	panic("purego: struct arguments are not supported")
}

func getStruct(outType reflect.Type, syscall syscall15Args) (v reflect.Value) {
	panic("purego: struct returns are not supported")
}

func placeRegisters(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) {
	panic("purego: placeRegisters not implemented on 386")
}

// shouldBundleStackArgs always returns false on 386
// since C-style stack argument bundling is only needed on Darwin ARM64.
func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	return false
}

// structFitsInRegisters is not used on 386.
func structFitsInRegisters(val reflect.Value, tempNumInts, tempNumFloats int) (bool, int, int) {
	panic("purego: structFitsInRegisters should not be called on 386")
}

// collectStackArgs is not used on 386.
func collectStackArgs(args []reflect.Value, startIdx int, numInts, numFloats int,
	keepAlive []any, addInt, addFloat, addStack func(uintptr),
	pNumInts, pNumFloats, pNumStack *int) ([]reflect.Value, []any) {
	panic("purego: collectStackArgs should not be called on 386")
}

// bundleStackArgs is not used on 386.
func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	panic("purego: bundleStackArgs should not be called on 386")
}
