// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024 The Ebitengine Authors

package purego

import (
	"math"
	"reflect"
	"runtime"
	"unsafe"
)

// structReturnInMemory reports whether a struct return value of the given size
// is returned through a caller-allocated hidden pointer passed as the first
// integer argument (true) rather than in registers (false).
func structReturnInMemory(outType reflect.Type) bool {
	size := outType.Size()
	if size == 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		// The Win64 ABI returns aggregates of exactly 1, 2, 4, or 8 bytes in
		// RAX. Every other size is returned through a caller-allocated hidden
		// pointer that the callee also returns in RAX.
		switch size {
		case 1, 2, 4, 8:
			return false
		default:
			return true
		}
	}
	// The System V ABI returns aggregates of up to two eightbytes in registers.
	return size > maxRegAllocStructSize
}

func getStruct(outType reflect.Type, syscall syscallArgs) (v reflect.Value) {
	outSize := outType.Size()
	if runtime.GOOS == "windows" {
		switch {
		case outSize == 0:
			return reflect.New(outType).Elem()
		case structReturnInMemory(outType):
			// Returned through the caller-allocated hidden pointer, which the
			// callee also returns in RAX.
			return reflect.NewAt(outType, *(*unsafe.Pointer)(unsafe.Pointer(&syscall.a1))).Elem()
		default:
			// 1, 2, 4, or 8 byte aggregates are returned in RAX.
			return reflect.NewAt(outType, unsafe.Pointer(&struct{ a uintptr }{syscall.a1})).Elem()
		}
	}
	switch {
	case outSize == 0:
		return reflect.New(outType).Elem()
	case outSize <= 8:
		if isAllFloats(outType) {
			// 2 float32s or 1 float64s are return in the float register
			return reflect.NewAt(outType, unsafe.Pointer(&struct{ a uintptr }{syscall.f1})).Elem()
		}
		// up to 8 bytes is returned in RAX
		return reflect.NewAt(outType, unsafe.Pointer(&struct{ a uintptr }{syscall.a1})).Elem()
	case outSize <= 16:
		r1, r2 := syscall.a1, syscall.a2
		if isAllFloats(outType) {
			r1 = syscall.f1
			r2 = syscall.f2
		} else {
			// check first 8 bytes if it's floats
			hasFirstFloat := false
			numFields := numABIFields(outType)
			f1 := abiField(outType, 0).Type
			if f1.Kind() == reflect.Float64 || f1.Kind() == reflect.Float32 && abiField(outType, 1).Type.Kind() == reflect.Float32 {
				r1 = syscall.f1
				hasFirstFloat = true
			}

			// find index of the field that starts the second 8 bytes
			var i int
			for i = 0; i < numFields; i++ {
				if abiField(outType, i).Offset == 8 {
					break
				}
			}

			// check last 8 bytes if they are floats
			f1 = abiField(outType, i).Type
			if f1.Kind() == reflect.Float64 || f1.Kind() == reflect.Float32 && i+1 == numFields {
				r2 = syscall.f1
			} else if hasFirstFloat {
				// if the first field was a float then that means the second integer field
				// comes from the first integer register
				r2 = syscall.a1
			}
		}
		return reflect.NewAt(outType, unsafe.Pointer(&struct{ a, b uintptr }{r1, r2})).Elem()
	default:
		// create struct from the Go pointer created above
		// weird pointer dereference to circumvent go vet
		return reflect.NewAt(outType, *(*unsafe.Pointer)(unsafe.Pointer(&syscall.a1))).Elem()
	}
}

func isAllFloats(ty reflect.Type) bool {
	for i := range ty.NumField() {
		f := ty.Field(i)
		if !isABIField(f) {
			continue
		}
		switch f.Type.Kind() {
		case reflect.Float64, reflect.Float32:
		default:
			return false
		}
	}
	return true
}

// https://refspecs.linuxbase.org/elf/x86_64-abi-0.99.pdf
// https://gitlab.com/x86-psABIs/x86-64-ABI
// Class determines where the 8 byte value goes.
// Higher value classes win over lower value classes
const (
	_NO_CLASS = 0b0000
	_SSE      = 0b0001
	_X87      = 0b0011 // long double not used in Go
	_INTEGER  = 0b0111
	_MEMORY   = 0b1111
)

func addStruct(v reflect.Value, numInts, numFloats, numStack *int, addInt, addFloat, addStack func(uintptr), keepAlive []any) []any {
	if runtime.GOOS == "windows" {
		// Win64 still passes an empty struct as an argument slot, so this must
		// run before the zero-size early return used by the System V path.
		return addStructWindows(v, addInt, keepAlive)
	}

	if v.Type().Size() == 0 {
		return keepAlive
	}

	// if greater than 64 bytes place on stack
	if v.Type().Size() > 8*8 {
		placeStack(v, addStack)
		return keepAlive
	}
	var (
		savedNumFloats = *numFloats
		savedNumInts   = *numInts
		savedNumStack  = *numStack
	)
	placeOnStack := postMerger(v.Type()) || !tryPlaceRegister(v, addFloat, addInt)
	if placeOnStack {
		// reset any values placed in registers
		*numFloats = savedNumFloats
		*numInts = savedNumInts
		*numStack = savedNumStack
		placeStack(v, addStack)
	}
	return keepAlive
}

// addStructWindows passes a struct argument under the Win64 ABI. Aggregates of
// exactly 1, 2, 4, or 8 bytes are passed by value in a single integer slot; all
// other sizes are passed as a pointer to a caller-allocated copy. Empty structs
// fall in the latter group: unlike the System V ABI, Win64 still consumes an
// argument slot for them.
func addStructWindows(v reflect.Value, addInt func(uintptr), keepAlive []any) []any {
	switch v.Type().Size() {
	case 1, 2, 4, 8:
		var val uintptr
		reflect.NewAt(v.Type(), unsafe.Pointer(&val)).Elem().Set(v)
		addInt(val)
	default:
		ptrStruct := reflect.New(v.Type())
		ptrStruct.Elem().Set(v)
		ptr := ptrStruct.Elem().Addr().UnsafePointer()
		keepAlive = append(keepAlive, ptr)
		addInt(uintptr(ptr))
	}
	return keepAlive
}

func postMerger(t reflect.Type) (passInMemory bool) {
	// (c) If the size of the aggregate exceeds two eightbytes and the first eight- byte isn’t SSE or any other
	// eightbyte isn’t SSEUP, the whole argument is passed in memory.
	if t.Kind() != reflect.Struct {
		return false
	}
	if t.Size() <= 2*8 {
		return false
	}
	return true // Go does not have an SSE/SSEUP type so this is always true
}

func tryPlaceRegister(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) (ok bool) {
	ok = true
	var val uint64
	var shift byte // # of bits to shift
	var flushed bool
	class := _NO_CLASS
	flushIfNeeded := func() {
		if flushed {
			return
		}
		flushed = true
		if class == _SSE {
			addFloat(uintptr(val))
		} else {
			addInt(uintptr(val))
		}
		val = 0
		shift = 0
		class = _NO_CLASS
	}
	var place func(v reflect.Value)
	place = func(v reflect.Value) {
		var numFields int
		if v.Kind() == reflect.Struct {
			numFields = v.Type().NumField()
		} else {
			numFields = v.Type().Len()
		}

		for i := range numFields {
			if v.Kind() == reflect.Struct && !isABIField(v.Type().Field(i)) {
				continue
			}
			flushed = false
			var f reflect.Value
			if v.Kind() == reflect.Struct {
				f = v.Field(i)
			} else {
				f = v.Index(i)
			}
			switch f.Kind() {
			case reflect.Struct:
				place(f)
			case reflect.Bool:
				if f.Bool() {
					val |= 1 << shift
				}
				shift += 8
				class |= _INTEGER
			case reflect.Pointer, reflect.UnsafePointer:
				val = uint64(f.Pointer())
				shift = 64
				class = _INTEGER
			case reflect.Int8:
				val |= uint64(f.Int()&0xFF) << shift
				shift += 8
				class |= _INTEGER
			case reflect.Int16:
				val |= uint64(f.Int()&0xFFFF) << shift
				shift += 16
				class |= _INTEGER
			case reflect.Int32:
				val |= uint64(f.Int()&0xFFFF_FFFF) << shift
				shift += 32
				class |= _INTEGER
			case reflect.Int64, reflect.Int:
				val = uint64(f.Int())
				shift = 64
				class = _INTEGER
			case reflect.Uint8:
				val |= f.Uint() << shift
				shift += 8
				class |= _INTEGER
			case reflect.Uint16:
				val |= f.Uint() << shift
				shift += 16
				class |= _INTEGER
			case reflect.Uint32:
				val |= f.Uint() << shift
				shift += 32
				class |= _INTEGER
			case reflect.Uint64, reflect.Uint, reflect.Uintptr:
				val = f.Uint()
				shift = 64
				class = _INTEGER
			case reflect.Float32:
				val |= uint64(math.Float32bits(float32(f.Float()))) << shift
				shift += 32
				class |= _SSE
			case reflect.Float64:
				if v.Type().Size() > 16 {
					ok = false
					return
				}
				val = uint64(math.Float64bits(f.Float()))
				shift = 64
				class = _SSE
			case reflect.Array:
				place(f)
			default:
				panic("purego: unsupported kind " + f.Kind().String())
			}

			if shift == 64 {
				flushIfNeeded()
			} else if shift > 64 {
				// Should never happen, but may if we forget to reset shift after flush (or forget to flush),
				// better fall apart here, than corrupt arguments.
				panic("purego: tryPlaceRegisters shift > 64")
			}
		}
	}

	place(v)
	flushIfNeeded()
	return ok
}

func placeStack(v reflect.Value, addStack func(uintptr)) {
	// Copy the struct as a contiguous block of memory in eightbyte (8-byte)
	// chunks. The x86-64 ABI requires structs passed on the stack to be
	// laid out exactly as in memory, including padding and field packing
	// within eightbytes. Decomposing field-by-field would place each field
	// as a separate stack slot, breaking structs with mixed-type fields
	// that share an eightbyte (e.g. int32 + float32).
	if !v.CanAddr() {
		tmp := reflect.New(v.Type()).Elem()
		tmp.Set(v)
		v = tmp
	}
	ptr := v.Addr().UnsafePointer()
	size := v.Type().Size()
	for off := uintptr(0); off < size; off += 8 {
		chunk := *(*uintptr)(unsafe.Add(ptr, off))
		addStack(chunk)
	}
}

func placeRegisters(v reflect.Value, addFloat func(uintptr), addInt func(uintptr)) {
	panic("purego: placeRegisters not implemented on amd64")
}

// shouldBundleStackArgs always returns false on non-Darwin platforms
// since C-style stack argument bundling is only needed on Darwin ARM64.
func shouldBundleStackArgs(v reflect.Value, numInts, numFloats int) bool {
	return false
}

// structFitsInRegisters is not used on amd64.
func structFitsInRegisters(val reflect.Value, tempNumInts, tempNumFloats int) (bool, int, int) {
	panic("purego: structFitsInRegisters should not be called on amd64")
}

// collectStackArgs is not used on amd64.
func collectStackArgs(args []reflect.Value, startIdx int, numInts, numFloats int,
	keepAlive []any, addInt, addFloat, addStack func(uintptr),
	pNumInts, pNumFloats, pNumStack *int) ([]reflect.Value, []any) {
	panic("purego: collectStackArgs should not be called on amd64")
}

// bundleStackArgs is not used on amd64.
func bundleStackArgs(stackArgs []reflect.Value, addStack func(uintptr)) {
	panic("purego: bundleStackArgs should not be called on amd64")
}

// getCallbackStruct reads a struct argument from the callback frame on amd64.
// It mirrors the SysV AMD64 ABI rules used by addStruct for the Go→C path.
//
// getCallbackStruct is only used on Unix. On Windows, callbacks are handled by
// the runtime's own callback mechanism, so this function is compiled but unused.
//
// SysV AMD64 struct argument passing rules:
//   - Struct > 16 bytes (postMerger): passed on the stack as raw bytes
//   - Struct ≤ 16 bytes: classify each eightbyte (INTEGER or SSE),
//     read from the appropriate register class
//   - If not enough registers for all eightbytes: entire struct goes on the stack
func getCallbackStruct(inType reflect.Type, frame unsafe.Pointer, floatsN *int, intsN *int, stackSlot *int, stackByteOffset *uintptr) reflect.Value {
	switch runtime.GOOS {
	case "android", "darwin", "freebsd", "ios", "linux", "netbsd":
	default:
		panic("purego: getCallbackStruct is not supported on " + runtime.GOOS)
	}

	f := (*[callbackMaxFrame]uintptr)(frame)
	size := inType.Size()

	// Structs > 16 bytes are passed on the stack as raw bytes (SysV ABI MEMORY class).
	if postMerger(inType) {
		numSlots := int((size + 7) / 8)
		v := reflect.NewAt(inType, unsafe.Pointer(&f[*stackSlot])).Elem()
		*stackSlot += numSlots
		return v
	}

	// Struct ≤ 16 bytes: classify each eightbyte and read from the appropriate register.
	numEightbytes := int((size + 7) / 8)

	// Count how many integer and SSE registers this struct needs.
	var needInts, needFloats int
	for i := range numEightbytes {
		class := classifyEightbyte(inType, uintptr(i)*8, uintptr(i)*8+8)
		if class == _SSE {
			needFloats++
		} else {
			needInts++
		}
	}

	// If not enough registers for all eightbytes, the entire struct goes on the stack.
	if *intsN+needInts > numOfIntegerRegisters() || *floatsN+needFloats > numOfFloatRegisters() {
		v := reflect.NewAt(inType, unsafe.Pointer(&f[*stackSlot])).Elem()
		*stackSlot += numEightbytes
		return v
	}

	// Read each eightbyte from its appropriate register class.
	var r1, r2 uintptr
	for i := range numEightbytes {
		class := classifyEightbyte(inType, uintptr(i)*8, uintptr(i)*8+8)
		if class == _SSE {
			if i == 0 {
				r1 = f[*floatsN]
			} else {
				r2 = f[*floatsN]
			}
			*floatsN++
		} else {
			if i == 0 {
				r1 = f[numOfFloatRegisters()+*intsN]
			} else {
				r2 = f[numOfFloatRegisters()+*intsN]
			}
			*intsN++
		}
	}

	if numEightbytes == 1 {
		return reflect.NewAt(inType, unsafe.Pointer(&struct{ a uintptr }{r1})).Elem()
	}
	return reflect.NewAt(inType, unsafe.Pointer(&struct{ a, b uintptr }{r1, r2})).Elem()
}

func setStruct(a *callbackArgs, ret reflect.Value) {
	outSize := ret.Type().Size()
	switch {
	case outSize == 0:
		return
	case outSize <= 16:
		// Copy the struct's raw bytes (including padding) into a buffer.
		var buf [2]uintptr
		reflect.NewAt(ret.Type(), unsafe.Pointer(&buf[0])).Elem().Set(ret)
		// Classify each eightbyte by the SysV ABI rules (§3.2.3, rule 4d
		// of https://refspecs.linuxbase.org/elf/x86_64-abi-0.99.pdf).
		// INTEGER wins over SSE for mixed eightbytes. Place INTEGER eightbytes in
		// result[0]/result[1] (AX/DX) and SSE eightbytes in result[2]/result[3]
		// (XMM0/XMM1).
		// Assign each eightbyte to the next available register of the
		// appropriate class. The ABI counts integer (AX, DX) and SSE
		// (XMM0, XMM1) return registers independently.
		var numInts int
		var numFloats int
		for i := 0; i < 2 && uintptr(i)*8 < outSize; i++ {
			class := classifyEightbyte(ret.Type(), uintptr(i)*8, uintptr(i)*8+8)
			if class == _SSE {
				switch numFloats {
				case 0:
					a.result[2] = buf[i]
				case 1:
					a.result[3] = buf[i]
				}
				numFloats++
			} else {
				switch numInts {
				case 0:
					a.result[0] = buf[i]
				case 1:
					a.result[1] = buf[i]
				}
				numInts++
			}
		}
	default:
		// Structs > 16 bytes are returned by hidden pointer.
		// a.result[0] contains the pointer passed by the caller in RDI.
		// Write the struct through this pointer.
		reflect.NewAt(ret.Type(), *(*unsafe.Pointer)(unsafe.Pointer(&a.result[0]))).Elem().Set(ret)
	}
}

// classifyEightbyte returns the SysV ABI class for the byte range [start, end)
// within a type, by examining all scalar fields that overlap that range.
func classifyEightbyte(t reflect.Type, start, end uintptr) int {
	return doClassifyEightbyte(t, 0, start, end)
}

func doClassifyEightbyte(t reflect.Type, base, start, end uintptr) int {
	switch t.Kind() {
	case reflect.Struct:
		class := _NO_CLASS
		for i := range t.NumField() {
			f := t.Field(i)
			class |= doClassifyEightbyte(f.Type, base+f.Offset, start, end)
		}
		return class
	case reflect.Array:
		class := _NO_CLASS
		elemSize := t.Elem().Size()
		for i := range t.Len() {
			class |= doClassifyEightbyte(t.Elem(), base+uintptr(i)*elemSize, start, end)
		}
		return class
	default:
		fStart := base
		fEnd := base + t.Size()
		if fStart >= end || fEnd <= start {
			return _NO_CLASS
		}
		switch t.Kind() {
		case reflect.Float32, reflect.Float64:
			return _SSE
		default:
			return _INTEGER
		}
	}
}
