// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

package purego

import (
	"reflect"
	"unsafe"
)

func getStruct(outType reflect.Type, syscall syscall15Args) reflect.Value {
	outSize := outType.Size()

	switch {
	case outSize == 0:
		return reflect.New(outType).Elem()

	case outSize <= 16:
		// Reconstruct from registers by copying raw bytes
		var buf [16]byte

		// Integer registers
		*(*uintptr)(unsafe.Pointer(&buf[0])) = syscall.a1
		if outSize > 8 {
			*(*uintptr)(unsafe.Pointer(&buf[8])) = syscall.a2
		}

		// Homogeneous float aggregates override integer regs
		if isAllFloats, numFields := isAllSameFloat(outType); isAllFloats {
			if outType.Field(0).Type.Kind() == reflect.Float32 {
				// float32 values in FP regs
				f := []uintptr{syscall.f1, syscall.f2, syscall.f3, syscall.f4}
				for i := 0; i < numFields; i++ {
					*(*uint32)(unsafe.Pointer(&buf[i*4])) = uint32(f[i])
				}
			} else {
				// float64: whole register value is valid
				*(*uintptr)(unsafe.Pointer(&buf[0])) = syscall.f1
				if outSize > 8 {
					*(*uintptr)(unsafe.Pointer(&buf[8])) = syscall.f2
				}
			}
		}

		return reflect.NewAt(outType, unsafe.Pointer(&buf[0])).Elem()

	default:
		// Returned indirectly via pointer in a1
		ptr := *(*unsafe.Pointer)(unsafe.Pointer(&syscall.a1))
		return reflect.NewAt(outType, ptr).Elem()
	}
}

func addStruct(
	v reflect.Value,
	numInts, numFloats, numStack *int,
	addInt, addFloat, addStack func(uintptr),
	keepAlive []any,
) []any {
	size := v.Type().Size()
	if size == 0 {
		return keepAlive
	}

	if size <= 16 {
		return placeSmallAggregateS390X(v, addFloat, addInt, keepAlive)
	}

	return placeStack(v, keepAlive, addInt)
}

func placeSmallAggregateS390X(
	v reflect.Value,
	addFloat, addInt func(uintptr),
	keepAlive []any,
) []any {
	size := v.Type().Size()

	var ptr unsafe.Pointer
	if v.CanAddr() {
		ptr = v.Addr().UnsafePointer()
	} else {
		tmp := reflect.New(v.Type())
		tmp.Elem().Set(v)
		ptr = tmp.UnsafePointer()
		keepAlive = append(keepAlive, tmp.Interface())
	}

	var buf [16]byte
	src := unsafe.Slice((*byte)(ptr), size)
	copy(buf[:], src)

	w0 := *(*uintptr)(unsafe.Pointer(&buf[0]))
	w1 := uintptr(0)
	if size > 8 {
		w1 = *(*uintptr)(unsafe.Pointer(&buf[8]))
	}

	if isFloats, _ := isAllSameFloat(v.Type()); isFloats {
		addFloat(w0)
		if size > 8 {
			addFloat(w1)
		}
	} else {
		addInt(w0)
		if size > 8 {
			addInt(w1)
		}
	}

	return keepAlive
}

// placeStack is a fallback for structs that are too large to fit in registers
func placeStack(v reflect.Value, keepAlive []any, addInt func(uintptr)) []any {
	if v.CanAddr() {
		addInt(v.Addr().Pointer())
		return keepAlive
	}
	ptr := reflect.New(v.Type())
	ptr.Elem().Set(v)
	addInt(ptr.Pointer())
	return append(keepAlive, ptr.Interface())
}

func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	// S390X does not bundle stack args
	return false
}

func collectStackArgs(
	args []reflect.Value,
	i, numInts, numFloats int,
	keepAlive []any,
	addInt, addFloat, addStack func(uintptr),
	numIntsPtr, numFloatsPtr, numStackPtr *int,
) ([]reflect.Value, []any) {
	return nil, keepAlive
}

func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	panic("bundleStackArgs not supported on S390X")
}
