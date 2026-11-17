// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package purego

import (
	"math"
	"reflect"
	"runtime"
	"strconv"
	stdstrings "strings"
	"unsafe"

	"github.com/ebitengine/purego/internal/strings"
)

// structReturnInMemory always reports false on arm64: a struct returned in
// memory is passed through the dedicated indirect result register (R8), not as
// an ordinary integer argument, so it is handled separately.
func structReturnInMemory(reflect.Type) bool {
	return false
}

func getStruct(outType reflect.Type, syscall syscallArgs) (v reflect.Value) {
	outSize := outType.Size()
	switch {
	case outSize == 0:
		return reflect.New(outType).Elem()
	case outSize <= 8:
		r1 := syscall.a1
		if isAllFloats, numFields := isAllSameFloat(outType); isAllFloats {
			r1 = syscall.f1
			if numFields == 2 {
				r1 = syscall.f2<<32 | syscall.f1&math.MaxUint32
			}
		}
		return reflect.NewAt(outType, unsafe.Pointer(&struct{ a uintptr }{r1})).Elem()
	case outSize <= 16:
		r1, r2 := syscall.a1, syscall.a2
		if isAllFloats, numFields := isAllSameFloat(outType); isAllFloats {
			switch numFields {
			case 4:
				r1 = syscall.f2<<32 | syscall.f1&math.MaxUint32
				r2 = syscall.f4<<32 | syscall.f3&math.MaxUint32
			case 3:
				r1 = syscall.f2<<32 | syscall.f1&math.MaxUint32
				r2 = syscall.f3
			case 2:
				r1 = syscall.f1
				r2 = syscall.f2
			default:
				panic("not reached")
			}
		}
		return reflect.NewAt(outType, unsafe.Pointer(&struct{ a, b uintptr }{r1, r2})).Elem()
	default:
		if isAllFloats, numFields := isAllSameFloat(outType); isAllFloats && numFields <= 4 {
			switch numFields {
			case 4:
				return reflect.NewAt(outType, unsafe.Pointer(&struct{ a, b, c, d uintptr }{syscall.f1, syscall.f2, syscall.f3, syscall.f4})).Elem()
			case 3:
				return reflect.NewAt(outType, unsafe.Pointer(&struct{ a, b, c uintptr }{syscall.f1, syscall.f2, syscall.f3})).Elem()
			default:
				panic("not reached")
			}
		}
		// create struct from the Go pointer created in arm64_r8
		// weird pointer dereference to circumvent go vet
		return reflect.NewAt(outType, *(*unsafe.Pointer)(unsafe.Pointer(&syscall.arm64_r8))).Elem()
	}
}

// https://github.com/ARM-software/abi-aa/blob/main/sysvabi64/sysvabi64.rst
const (
	_NO_CLASS = 0b00
	_FLOAT    = 0b01
	_INT      = 0b11
)

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []any) []any {
	if v.Type().Size() == 0 {
		return keepAlive
	}

	if hva, hfa, size := isHVA(v.Type()), isHFA(v.Type()), v.Type().Size(); hva || hfa || size <= 16 {
		// if this doesn't fit entirely in registers then
		// each element goes onto the stack
		if hfa && *numFloats+numABIFields(v.Type()) > numOfFloatRegisters() {
			*numFloats = numOfFloatRegisters()
		} else if hva && *numInts+numABIFields(v.Type()) > numOfIntegerRegisters() {
			*numInts = numOfIntegerRegisters()
		}

		placeRegisters(v, addFloat, addInt)
	} else {
		keepAlive = placeStack(v, keepAlive, addInt)
	}
	return keepAlive // the struct was allocated so don't panic
}

func placeRegisters(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) {
	if isDarwin {
		placeRegistersDarwin(v, addFloat, addInt)
		return
	}
	placeRegistersArm64(v, addFloat, addInt)
}

func placeRegistersArm64(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) {
	var val uint64
	var shift byte
	var flushed bool
	class := _NO_CLASS
	var place func(v reflect.Value)
	place = func(v reflect.Value) {
		var numFields int
		if v.Kind() == reflect.Struct {
			numFields = v.Type().NumField()
		} else {
			numFields = v.Type().Len()
		}
		for k := range numFields {
			if v.Kind() == reflect.Struct && !isABIField(v.Type().Field(k)) {
				continue
			}
			flushed = false
			var f reflect.Value
			if v.Kind() == reflect.Struct {
				f = v.Field(k)
			} else {
				f = v.Index(k)
			}
			align := byte(f.Type().Align()*8 - 1)
			shift = (shift + align) &^ align
			if shift >= 64 {
				shift = 0
				flushed = true
				if class == _FLOAT {
					addFloat(uintptr(val))
				} else {
					addInt(uintptr(val))
				}
				val = 0
				class = _NO_CLASS
			}
			switch f.Type().Kind() {
			case reflect.Struct:
				place(f)
			case reflect.Bool:
				if f.Bool() {
					val |= 1 << shift
				}
				shift += 8
				class |= _INT
			case reflect.Uint8:
				val |= f.Uint() << shift
				shift += 8
				class |= _INT
			case reflect.Uint16:
				val |= f.Uint() << shift
				shift += 16
				class |= _INT
			case reflect.Uint32:
				val |= f.Uint() << shift
				shift += 32
				class |= _INT
			case reflect.Uint64, reflect.Uint, reflect.Uintptr:
				addInt(uintptr(f.Uint()))
				shift = 0
				flushed = true
				class = _NO_CLASS
			case reflect.Int8:
				val |= uint64(f.Int()&0xFF) << shift
				shift += 8
				class |= _INT
			case reflect.Int16:
				val |= uint64(f.Int()&0xFFFF) << shift
				shift += 16
				class |= _INT
			case reflect.Int32:
				val |= uint64(f.Int()&0xFFFF_FFFF) << shift
				shift += 32
				class |= _INT
			case reflect.Int64, reflect.Int:
				addInt(uintptr(f.Int()))
				shift = 0
				flushed = true
				class = _NO_CLASS
			case reflect.Float32:
				if class == _FLOAT {
					addFloat(uintptr(val))
					val = 0
					shift = 0
				}
				val |= uint64(math.Float32bits(float32(f.Float()))) << shift
				shift += 32
				class |= _FLOAT
			case reflect.Float64:
				addFloat(uintptr(math.Float64bits(float64(f.Float()))))
				shift = 0
				flushed = true
				class = _NO_CLASS
			case reflect.Pointer, reflect.UnsafePointer:
				addInt(f.Pointer())
				shift = 0
				flushed = true
				class = _NO_CLASS
			case reflect.Array:
				place(f)
			default:
				panic("purego: unsupported kind " + f.Kind().String())
			}
		}
	}
	place(v)
	if !flushed {
		if class == _FLOAT {
			addFloat(uintptr(val))
		} else {
			addInt(uintptr(val))
		}
	}
}

func placeStack(v reflect.Value, keepAlive []any, addInt func(uintptr)) []any {
	// Struct is too big to be placed in registers.
	// Copy to heap and place the pointer in register
	ptrStruct := reflect.New(v.Type())
	ptrStruct.Elem().Set(v)
	ptr := ptrStruct.Elem().Addr().UnsafePointer()
	keepAlive = append(keepAlive, ptr)
	addInt(uintptr(ptr))
	return keepAlive
}

// isHFA reports a Homogeneous Floating-point Aggregate (HFA) which is a Fundamental Data Type that is a
// Floating-Point type and at most four uniquely addressable members (5.9.5.1 in [Arm64 Calling Convention]).
// This type of struct will be placed more compactly than the individual fields.
//
// [Arm64 Calling Convention]: https://github.com/ARM-software/abi-aa/blob/main/sysvabi64/sysvabi64.rst
func isHFA(t reflect.Type) bool {
	// round up struct size to nearest 8 see section B.4
	structSize := roundUpTo8(t.Size())
	numFields := numABIFields(t)
	if structSize == 0 || numFields > 4 {
		return false
	}
	first := abiField(t, 0)
	switch first.Type.Kind() {
	case reflect.Float32, reflect.Float64:
		firstKind := first.Type.Kind()
		for i := range numFields {
			if abiField(t, i).Type.Kind() != firstKind {
				return false
			}
		}
		return true
	case reflect.Array:
		switch first.Type.Elem().Kind() {
		case reflect.Float32, reflect.Float64:
			return true
		default:
			return false
		}
	case reflect.Struct:
		for range numABIFields(first.Type) {
			if !isHFA(first.Type) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// isHVA reports a Homogeneous Aggregate with a Fundamental Data Type that is a Short-Vector type
// and at most four uniquely addressable members (5.9.5.2 in [Arm64 Calling Convention]).
// A short vector is a machine type that is composed of repeated instances of one fundamental integral or
// floating-point type. It may be 8 or 16 bytes in total size (5.4 in [Arm64 Calling Convention]).
// This type of struct will be placed more compactly than the individual fields.
//
// [Arm64 Calling Convention]: https://github.com/ARM-software/abi-aa/blob/main/sysvabi64/sysvabi64.rst
func isHVA(t reflect.Type) bool {
	// round up struct size to nearest 8 see section B.4
	structSize := roundUpTo8(t.Size())
	if structSize == 0 || (structSize != 8 && structSize != 16) {
		return false
	}
	numFields := numABIFields(t)
	first := abiField(t, 0)
	switch first.Type.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Int8, reflect.Int16, reflect.Int32:
		firstKind := first.Type.Kind()
		for i := range numFields {
			if abiField(t, i).Type.Kind() != firstKind {
				return false
			}
		}
		return true
	case reflect.Array:
		switch first.Type.Elem().Kind() {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Int8, reflect.Int16, reflect.Int32:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// copyStruct8ByteChunks copies struct memory in 8-byte chunks to the provided callback.
// This is used for Darwin ARM64's byte-level packing of non-HFA/HVA structs.
func copyStruct8ByteChunks(ptr unsafe.Pointer, size uintptr, addChunk func(uintptr)) {
	if !isDarwin {
		panic("purego: should only be called on darwin")
	}
	for offset := uintptr(0); offset < size; offset += 8 {
		var chunk uintptr
		remaining := size - offset
		if remaining >= 8 {
			chunk = *(*uintptr)(unsafe.Add(ptr, offset))
		} else {
			// Read byte-by-byte to avoid reading beyond allocation
			for i := range remaining {
				b := *(*byte)(unsafe.Add(ptr, offset+i))
				chunk |= uintptr(b) << (i * 8)
			}
		}
		addChunk(chunk)
	}
}

// placeRegisters implements Darwin ARM64 calling convention for struct arguments.
//
// For HFA/HVA structs, each element must go in a separate register (or stack slot for elements
// that don't fit in registers). We use placeRegistersArm64 for this.
//
// For non-HFA/HVA structs, Darwin uses byte-level packing. We copy the struct memory in
// 8-byte chunks, which works correctly for both register and stack placement.
func placeRegistersDarwin(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) {
	if !isDarwin {
		panic("purego: placeRegistersDarwin should only be called on darwin")
	}
	// Check if this is an HFA/HVA
	hfa := isHFA(v.Type())
	hva := isHVA(v.Type())

	// For HFA/HVA structs, use the standard ARM64 logic which places each element separately
	if hfa || hva {
		placeRegistersArm64(v, addFloat, addInt)
		return
	}

	// For non-HFA/HVA structs, use byte-level copying
	// If the value is not addressable, create an addressable copy
	if !v.CanAddr() {
		addressable := reflect.New(v.Type()).Elem()
		addressable.Set(v)
		v = addressable
	}
	ptr := unsafe.Pointer(v.Addr().Pointer())
	size := v.Type().Size()
	copyStruct8ByteChunks(ptr, size, addInt)
}

// shouldBundleStackArgs determines if we need to start C-style packing for
// Darwin ARM64 stack arguments. This happens when registers are exhausted.
func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	if !isDarwin {
		return false
	}

	kind := v.Kind()
	isFloat := kind == reflect.Float32 || kind == reflect.Float64
	isInt := !isFloat && kind != reflect.Struct
	primitiveOnStack :=
		(isInt && numInts >= numOfIntegerRegisters()) ||
			(isFloat && numFloats >= numOfFloatRegisters())
	if primitiveOnStack {
		return true
	}
	if kind != reflect.Struct {
		return false
	}
	hfa := isHFA(v.Type())
	hva := isHVA(v.Type())
	size := v.Type().Size()
	eligible := hfa || hva || size <= 16
	if !eligible {
		return false
	}

	if hfa {
		need := numABIFields(v.Type())
		return numFloats+need > numOfFloatRegisters()
	}

	if hva {
		need := numABIFields(v.Type())
		return numInts+need > numOfIntegerRegisters()
	}

	slotsNeeded := int((size + align8ByteMask) / align8ByteSize)
	return numInts+slotsNeeded > numOfIntegerRegisters()
}

// structFitsInRegisters determines if a struct can still fit in remaining
// registers, used during stack argument bundling to decide if a struct
// should go through normal register allocation or be bundled with stack args.
func structFitsInRegisters(val reflect.Value, tempNumInts, tempNumFloats int) (bool, int, int) {
	if !isDarwin {
		panic("purego: structFitsInRegisters should only be called on darwin")
	}
	hfa := isHFA(val.Type())
	hva := isHVA(val.Type())
	size := val.Type().Size()

	if hfa {
		// HFA: check if elements fit in float registers
		if numFields := numABIFields(val.Type()); tempNumFloats+numFields <= numOfFloatRegisters() {
			return true, tempNumInts, tempNumFloats + numFields
		}
	} else if hva {
		// HVA: check if elements fit in int registers
		if numFields := numABIFields(val.Type()); tempNumInts+numFields <= numOfIntegerRegisters() {
			return true, tempNumInts + numFields, tempNumFloats
		}
	} else if size <= 16 {
		// Non-HFA/HVA small structs use int registers for byte-packing
		slotsNeeded := int((size + align8ByteMask) / align8ByteSize)
		if tempNumInts+slotsNeeded <= numOfIntegerRegisters() {
			return true, tempNumInts + slotsNeeded, tempNumFloats
		}
	}

	return false, tempNumInts, tempNumFloats
}

// collectStackArgs separates remaining arguments into those that fit in registers vs those that go on stack.
// It returns the stack arguments and processes register arguments through addValue.
func collectStackArgs(args []reflect.Value, startIdx int, numInts, numFloats int,
	keepAlive []any, addInt, addFloat, addStack func(uintptr),
	pNumInts, pNumFloats, pNumStack *int) ([]reflect.Value, []any) {
	if !isDarwin {
		panic("purego: collectStackArgs should only be called on darwin")
	}

	var stackArgs []reflect.Value
	tempNumInts := numInts
	tempNumFloats := numFloats

	for j, val := range args[startIdx:] {
		// Determine if this argument goes to register or stack
		var fitsInRegister bool
		var newNumInts, newNumFloats int

		if val.Kind() == reflect.Struct {
			// Check if struct still fits in remaining registers
			fitsInRegister, newNumInts, newNumFloats = structFitsInRegisters(val, tempNumInts, tempNumFloats)
		} else {
			// Primitive argument
			isFloat := val.Kind() == reflect.Float32 || val.Kind() == reflect.Float64
			if isFloat {
				fitsInRegister = tempNumFloats < numOfFloatRegisters()
				newNumFloats = tempNumFloats + 1
				newNumInts = tempNumInts
			} else {
				fitsInRegister = tempNumInts < numOfIntegerRegisters()
				newNumInts = tempNumInts + 1
				newNumFloats = tempNumFloats
			}
		}

		if fitsInRegister {
			// Process through normal register allocation
			tempNumInts = newNumInts
			tempNumFloats = newNumFloats
			keepAlive = addValue(val, keepAlive, addInt, addFloat, addStack, pNumInts, pNumFloats, pNumStack)
		} else {
			// Convert strings to C strings before bundling
			if val.Kind() == reflect.String {
				ptr := strings.CString(val.String())
				keepAlive = append(keepAlive, ptr)
				val = reflect.ValueOf(ptr)
				args[startIdx+j] = val
			}
			stackArgs = append(stackArgs, val)
		}
	}

	return stackArgs, keepAlive
}

const (
	paddingFieldPrefix = "Pad"
)

// bundleStackArgs bundles remaining arguments for Darwin ARM64 C-style stack packing.
// It creates a packed struct with proper alignment and copies it to the stack in 8-byte chunks.
func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	if !isDarwin {
		panic("purego: bundleStackArgs should only be called on darwin")
	}
	if len(stackArgs) == 0 {
		return
	}

	// Build struct fields with proper C alignment and padding
	var fields []reflect.StructField
	currentOffset := uintptr(0)
	fieldIndex := 0

	for j, val := range stackArgs {
		valSize := val.Type().Size()
		valAlign := val.Type().Align()

		// ARM64 requires 8-byte alignment for 8-byte or larger structs
		if val.Kind() == reflect.Struct && valSize >= 8 {
			valAlign = 8
		}

		// Add padding field if needed for alignment
		if currentOffset%uintptr(valAlign) != 0 {
			paddingNeeded := uintptr(valAlign) - (currentOffset % uintptr(valAlign))
			fields = append(fields, reflect.StructField{
				Name: paddingFieldPrefix + strconv.Itoa(fieldIndex),
				Type: reflect.ArrayOf(int(paddingNeeded), reflect.TypeFor[byte]()),
			})
			currentOffset += paddingNeeded
			fieldIndex++
		}

		fields = append(fields, reflect.StructField{
			Name: "X" + strconv.Itoa(j),
			Type: val.Type(),
		})
		currentOffset += valSize
		fieldIndex++
	}

	// Create and populate the packed struct
	structType := reflect.StructOf(fields)
	structInstance := reflect.New(structType).Elem()

	// Set values (skip padding fields)
	argIndex := 0
	for j := range structInstance.NumField() {
		fieldName := structType.Field(j).Name
		if stdstrings.HasPrefix(fieldName, paddingFieldPrefix) {
			continue
		}
		structInstance.Field(j).Set(stackArgs[argIndex])
		argIndex++
	}

	ptr := unsafe.Pointer(structInstance.Addr().Pointer())
	size := structType.Size()
	copyStruct8ByteChunks(ptr, size, addStack)
}

// getCallbackStruct reads a struct argument from the callback frame on arm64.
// It mirrors the AAPCS64 rules used by addStruct for the Go→C path.
//
// getCallbackStruct is only used on Unix. On Windows, callbacks are handled by
// the runtime's own callback mechanism, so this function is compiled but unused.
//
// AAPCS64 struct argument passing rules:
//   - HFA (Homogeneous Float Aggregate): each float field in a separate float register
//   - Non-HFA ≤ 16 bytes: packed into 1–2 integer registers
//   - > 16 bytes (not HFA): passed by pointer in integer register
//   - Register overflow: struct goes on the stack
//   - Darwin ARM64: byte-level packing on the stack
func getCallbackStruct(inType reflect.Type, frame unsafe.Pointer, floatsN *int, intsN *int, stackSlot *int, stackByteOffset *uintptr) reflect.Value {
	switch runtime.GOOS {
	case "android", "darwin", "freebsd", "ios", "linux", "netbsd":
	default:
		panic("purego: getCallbackStruct is not supported on " + runtime.GOOS)
	}

	f := (*[callbackMaxFrame]uintptr)(frame)

	if isHFA(inType) {
		_, numFloatFields := isAllSameFloat(inType)
		if *floatsN+numFloatFields <= numOfFloatRegisters() {
			return readHFAFromRegisters(inType, f, floatsN, numFloatFields)
		}
		// Not enough float registers: entire struct goes on stack.
		// Exhaust float registers per AAPCS64.
		*floatsN = numOfFloatRegisters()
		return readStructFromStackArm64(inType, f, frame, stackSlot, stackByteOffset)
	}

	if size := inType.Size(); size <= 16 {
		// Non-HFA composite ≤ 16 bytes: packed into 1–2 integer registers.
		numSlots := int((size + 7) / 8)
		if *intsN+numSlots <= numOfIntegerRegisters() {
			pos := numOfFloatRegisters() + *intsN
			var r1, r2 uintptr
			r1 = f[pos]
			*intsN++
			if numSlots == 2 {
				r2 = f[pos+1]
				*intsN++
			}
			if numSlots == 1 {
				return reflect.NewAt(inType, unsafe.Pointer(&struct{ a uintptr }{r1})).Elem()
			}
			return reflect.NewAt(inType, unsafe.Pointer(&struct{ a, b uintptr }{r1, r2})).Elem()
		}
		// Not enough int registers: goes on stack.
		return readStructFromStackArm64(inType, f, frame, stackSlot, stackByteOffset)
	}

	// > 16 bytes, not HFA: passed by pointer in integer register.
	if *intsN < numOfIntegerRegisters() {
		ptr := f[numOfFloatRegisters()+*intsN]
		*intsN++
		return reflect.NewAt(inType, *(*unsafe.Pointer)(unsafe.Pointer(&ptr))).Elem()
	}

	// Pointer on stack (rare: all integer registers exhausted).
	if isDarwin {
		ptrVal := callbackArgFromStack(frame, *stackSlot, stackByteOffset, reflect.TypeFor[uintptr]())
		ptr := uintptr(ptrVal.Uint())
		return reflect.NewAt(inType, *(*unsafe.Pointer)(unsafe.Pointer(&ptr))).Elem()
	}
	ptr := f[*stackSlot]
	*stackSlot++
	return reflect.NewAt(inType, *(*unsafe.Pointer)(unsafe.Pointer(&ptr))).Elem()
}

// readHFAFromRegisters reads an HFA (Homogeneous Float Aggregate) struct from
// float registers. Each float field occupies one float register slot.
// For float32, the value is in the low 32 bits of the register.
func readHFAFromRegisters(inType reflect.Type, f *[callbackMaxFrame]uintptr, floatsN *int, numFields int) reflect.Value {
	size := inType.Size()

	// Determine the element type
	root := abiField(inType, 0).Type
	for root.Kind() == reflect.Struct {
		root = abiField(root, 0).Type
	}
	isFloat32 := root.Kind() == reflect.Float32

	if isFloat32 {
		// Each float32 is in a separate register (low 32 bits).
		// Pack pairs into uintptrs to match the struct memory layout.
		switch {
		case size <= 8:
			var r1 uintptr
			switch numFields {
			case 1:
				r1 = f[*floatsN]
			case 2:
				r1 = f[*floatsN+1]<<32 | f[*floatsN]&math.MaxUint32
			}
			*floatsN += numFields
			return reflect.NewAt(inType, unsafe.Pointer(&struct{ a uintptr }{r1})).Elem()
		case size <= 16:
			var r1, r2 uintptr
			switch numFields {
			case 3:
				r1 = f[*floatsN+1]<<32 | f[*floatsN]&math.MaxUint32
				r2 = f[*floatsN+2]
			case 4:
				r1 = f[*floatsN+1]<<32 | f[*floatsN]&math.MaxUint32
				r2 = f[*floatsN+3]<<32 | f[*floatsN+2]&math.MaxUint32
			}
			*floatsN += numFields
			return reflect.NewAt(inType, unsafe.Pointer(&struct{ a, b uintptr }{r1, r2})).Elem()
		default:
			panic("not reached")
		}
	}

	// float64: each field occupies one full register.
	switch numFields {
	case 1:
		r1 := f[*floatsN]
		*floatsN++
		return reflect.NewAt(inType, unsafe.Pointer(&struct{ a uintptr }{r1})).Elem()
	case 2:
		r1, r2 := f[*floatsN], f[*floatsN+1]
		*floatsN += 2
		return reflect.NewAt(inType, unsafe.Pointer(&struct{ a, b uintptr }{r1, r2})).Elem()
	case 3:
		r1, r2, r3 := f[*floatsN], f[*floatsN+1], f[*floatsN+2]
		*floatsN += 3
		return reflect.NewAt(inType, unsafe.Pointer(&struct{ a, b, c uintptr }{r1, r2, r3})).Elem()
	case 4:
		r1, r2, r3, r4 := f[*floatsN], f[*floatsN+1], f[*floatsN+2], f[*floatsN+3]
		*floatsN += 4
		return reflect.NewAt(inType, unsafe.Pointer(&struct{ a, b, c, d uintptr }{r1, r2, r3, r4})).Elem()
	default:
		panic("purego: HFA with more than 4 fields is not supported")
	}
}

// readStructFromStackArm64 reads a struct argument from the stack area of the
// callback frame. On Darwin ARM64, arguments are byte-packed on the stack.
// On Linux ARM64, arguments are 8-byte aligned.
func readStructFromStackArm64(inType reflect.Type, f *[callbackMaxFrame]uintptr, frame unsafe.Pointer, stackSlot *int, stackByteOffset *uintptr) reflect.Value {
	if isDarwin {
		return callbackArgFromStack(frame, *stackSlot, stackByteOffset, inType)
	}
	// Linux ARM64: 8-byte aligned slots.
	numSlots := int((inType.Size() + 7) / 8)
	v := reflect.NewAt(inType, unsafe.Pointer(&f[*stackSlot])).Elem()
	*stackSlot += numSlots
	return v
}

func setStruct(a *callbackArgs, ret reflect.Value) {
	outSize := ret.Type().Size()
	switch {
	case outSize == 0:
		return
	case outSize <= 8:
		reflect.NewAt(ret.Type(), unsafe.Pointer(&a.result)).Elem().Set(ret)
		if isAllFloats, numFields := isAllSameFloat(ret.Type()); isAllFloats && numFields == 2 {
			a.result[1] = a.result[0] >> 32 // expanding two float32s into a.result[0] and a.result[1]
			a.result[0] &= math.MaxUint32   // clear the top bits since they contain the second argument
		}
		return
	case outSize <= 16:
		reflect.NewAt(ret.Type(), unsafe.Pointer(&a.result)).Elem().Set(ret)
		if isAllFloats, numFields := isAllSameFloat(ret.Type()); isAllFloats {
			switch numFields {
			case 4:
				a.result[3] = a.result[1] >> 32
				a.result[2] = a.result[1] & math.MaxUint32
				a.result[1] = a.result[0] >> 32
				a.result[0] &= math.MaxUint32
			case 3:
				a.result[2] = a.result[1] & math.MaxUint32
				a.result[1] = a.result[0] >> 32
				a.result[0] &= math.MaxUint32
			case 2:
				// two float64s are already in a.result[0] and a.result[1]
			default:
				panic("not reached")
			}
		}
		return
	default:
		if isAllFloats, numFields := isAllSameFloat(ret.Type()); isAllFloats && numFields <= 4 {
			reflect.NewAt(ret.Type(), unsafe.Pointer(&a.result)).Elem().Set(ret)
			return
		}
		// The caller passed the address to place the return struct,
		// so copy the Go struct into the provided memory.
		reflect.NewAt(ret.Type(), *(*unsafe.Pointer)(unsafe.Pointer(&a.result[0]))).Elem().Set(ret)
		return
	}
}
