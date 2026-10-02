// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

package purego

import (
	"math"
	"reflect"
	"unsafe"
)

// ppc64leLeaf is a scalar member of a flattened aggregate at byte offset offset.
type ppc64leLeaf struct {
	kind   reflect.Kind
	offset uintptr
}

// ppc64leFlatten appends the scalar leaves of t to leaves, expanding nested
// structs and arrays.
func ppc64leFlatten(t reflect.Type, base uintptr, leaves *[]ppc64leLeaf) {
	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			if f.Name == "_" {
				// Blank fields are padding, not counted members.
				continue
			}
			ppc64leFlatten(f.Type, base+f.Offset, leaves)
		}
	case reflect.Array:
		elem := t.Elem()
		for i := range t.Len() {
			ppc64leFlatten(elem, base+uintptr(i)*elem.Size(), leaves)
		}
	default:
		*leaves = append(*leaves, ppc64leLeaf{kind: t.Kind(), offset: base})
	}
}

// ppc64leClassifyHFA reports whether t is a homogeneous floating-point
// aggregate: one to eight members all of the same floating-point type.
func ppc64leClassifyHFA(t reflect.Type) (leaves []ppc64leLeaf, isHFA bool) {
	ppc64leFlatten(t, 0, &leaves)
	if len(leaves) == 0 || len(leaves) > 8 {
		return leaves, false
	}
	first := leaves[0].kind
	if first != reflect.Float32 && first != reflect.Float64 {
		return leaves, false
	}
	for _, l := range leaves {
		if l.kind != first {
			return leaves, false
		}
	}
	return leaves, true
}

// structReturnInMemory reports whether a struct return value is returned through
// a caller-allocated hidden pointer rather than in registers. A homogeneous
// floating-point aggregate is returned in the floating-point registers whatever
// its size.
func structReturnInMemory(outType reflect.Type) bool {
	if outType.Size() <= maxRegAllocStructSize {
		return false
	}
	_, isHFA := ppc64leClassifyHFA(outType)
	return !isHFA
}

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []any) []any {
	size := v.Type().Size()
	if size == 0 {
		return keepAlive
	}

	var ptr unsafe.Pointer
	if v.CanAddr() {
		ptr = v.Addr().UnsafePointer()
	} else {
		tmp := reflect.New(v.Type())
		tmp.Elem().Set(v)
		ptr = tmp.UnsafePointer()
		keepAlive = append(keepAlive, tmp.Interface())
	}

	// An HFA passes each member in its own floating-point register.
	if leaves, isHFA := ppc64leClassifyHFA(v.Type()); isHFA {
		for _, l := range leaves {
			src := unsafe.Add(ptr, l.offset)
			if l.kind == reflect.Float32 {
				// Single precision occupies the register in double format.
				addFloat(uintptr(math.Float64bits(float64(*(*float32)(src)))))
			} else {
				addFloat(uintptr(*(*uint64)(src)))
			}
		}
	}

	// Every aggregate also occupies its size rounded up to doublewords in the
	// GPRs r3-r10 / parameter save area: shadow (skipped) for an HFA, the value
	// otherwise.
	for off := uintptr(0); off < size; off += 8 {
		if remaining := size - off; remaining >= 8 {
			addInt(*(*uintptr)(unsafe.Add(ptr, off)))
		} else {
			var word [8]byte
			copy(word[:], unsafe.Slice((*byte)(unsafe.Add(ptr, off)), int(remaining)))
			addInt(*(*uintptr)(unsafe.Pointer(&word[0])))
		}
	}

	return keepAlive
}

func getStruct(outType reflect.Type, syscall syscallArgs) reflect.Value {
	if outType.Size() == 0 {
		return reflect.New(outType).Elem()
	}

	// An HFA returns each member in its own floating-point register.
	if leaves, isHFA := ppc64leClassifyHFA(outType); isHFA {
		floatRegs := [8]uintptr{
			syscall.f1, syscall.f2, syscall.f3, syscall.f4,
			syscall.f5, syscall.f6, syscall.f7, syscall.f8,
		}
		ret := reflect.New(outType)
		base := ret.UnsafePointer()
		for i, l := range leaves {
			dst := unsafe.Add(base, l.offset)
			if l.kind == reflect.Float32 {
				// The register holds the value in double format.
				*(*float32)(dst) = float32(math.Float64frombits(uint64(floatRegs[i])))
			} else {
				*(*uint64)(dst) = uint64(floatRegs[i])
			}
		}
		return ret.Elem()
	}

	if outType.Size() > 16 {
		// Returned via a caller-allocated buffer whose pointer comes back in r3.
		return reflect.NewAt(outType, *(*unsafe.Pointer)(unsafe.Pointer(&syscall.a1))).Elem()
	}

	// Otherwise returned in r3 and r4.
	var buf [16]byte
	*(*uintptr)(unsafe.Pointer(&buf[0])) = syscall.a1
	if outType.Size() > 8 {
		*(*uintptr)(unsafe.Pointer(&buf[8])) = syscall.a2
	}
	return reflect.NewAt(outType, unsafe.Pointer(&buf[0])).Elem()
}

// shouldBundleStackArgs always returns false on ppc64le
// since C-style stack argument bundling is only needed on Darwin ARM64.
func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	return false
}

// collectStackArgs is not used on ppc64le.
func collectStackArgs(args []reflect.Value, startIdx int, numInts, numFloats int,
	keepAlive []any, addInt, addFloat, addStack func(uintptr),
	pNumInts, pNumFloats, pNumStack *int) ([]reflect.Value, []any) {
	panic("purego: collectStackArgs should not be called on ppc64le")
}

// bundleStackArgs is not used on ppc64le.
func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	panic("purego: bundleStackArgs should not be called on ppc64le")
}

func getCallbackStruct(inType reflect.Type, frame unsafe.Pointer, floatsN *int, intsN *int, stackSlot *int, stackByteOffset *uintptr) reflect.Value {
	panic("purego: struct callback arguments are not supported on ppc64le")
}

func setStruct(a *callbackArgs, ret reflect.Value) {
	panic("purego: struct returns are not supported on ppc64le")
}
