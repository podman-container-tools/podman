// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

// TODO: remove s390x cgo dependency once golang/go#77449 is resolved
//go:build darwin || freebsd || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || (cgo && s390x))) || netbsd

package purego

import (
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

var syscall15XABI0 uintptr

func syscall_syscall15X(fn, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr) (r1, r2, err uintptr) {
	args := thePool.Get().(*syscall15Args)
	defer thePool.Put(args)

	*args = syscall15Args{
		fn: fn,
		a1: a1, a2: a2, a3: a3, a4: a4, a5: a5, a6: a6, a7: a7, a8: a8,
		a9: a9, a10: a10, a11: a11, a12: a12, a13: a13, a14: a14, a15: a15,
		f1: a1, f2: a2, f3: a3, f4: a4, f5: a5, f6: a6, f7: a7, f8: a8,
	}

	runtime_cgocall(syscall15XABI0, unsafe.Pointer(args))
	return args.a1, args.a2, args.a3
}

// NewCallback converts a Go function to a function pointer conforming to the C calling convention.
// This is useful when interoperating with C code requiring callbacks. The argument is expected to be a
// function with zero or one uintptr-sized result. The function must not have arguments with size larger than the size
// of uintptr. Only a limited number of callbacks may be created in a single Go process, and any memory allocated
// for these callbacks is never released. At least 2000 callbacks can always be created. Although this function
// provides similar functionality to windows.NewCallback it is distinct.
func NewCallback(fn any) uintptr {
	ty := reflect.TypeOf(fn)
	for i := 0; i < ty.NumIn(); i++ {
		in := ty.In(i)
		if !in.AssignableTo(reflect.TypeOf(CDecl{})) {
			continue
		}
		if i != 0 {
			panic("purego: CDecl must be the first argument")
		}
	}
	return compileCallback(fn)
}

// maxCb is the maximum number of callbacks
// only increase this if you have added more to the callbackasm function
const maxCB = 2000

var cbs struct {
	lock  sync.Mutex
	numFn int                  // the number of functions currently in cbs.funcs
	funcs [maxCB]reflect.Value // the saved callbacks
}

func compileCallback(fn any) uintptr {
	val := reflect.ValueOf(fn)
	if val.Kind() != reflect.Func {
		panic("purego: the type must be a function but was not")
	}
	if val.IsNil() {
		panic("purego: function must not be nil")
	}
	ty := val.Type()
	for i := 0; i < ty.NumIn(); i++ {
		in := ty.In(i)
		switch in.Kind() {
		case reflect.Struct:
			if i == 0 && in.AssignableTo(reflect.TypeOf(CDecl{})) {
				continue
			}
			fallthrough
		case reflect.Interface, reflect.Func, reflect.Slice,
			reflect.Chan, reflect.Complex64, reflect.Complex128,
			reflect.String, reflect.Map, reflect.Invalid:
			panic("purego: unsupported argument type: " + in.Kind().String())
		}
	}
output:
	switch {
	case ty.NumOut() == 1:
		switch ty.Out(0).Kind() {
		case reflect.Pointer, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
			reflect.Bool, reflect.UnsafePointer:
			break output
		}
		panic("purego: unsupported return type: " + ty.String())
	case ty.NumOut() > 1:
		panic("purego: callbacks can only have one return")
	}
	cbs.lock.Lock()
	defer cbs.lock.Unlock()
	if cbs.numFn >= maxCB {
		panic("purego: the maximum number of callbacks has been reached")
	}
	cbs.funcs[cbs.numFn] = val
	cbs.numFn++
	return callbackasmAddr(cbs.numFn - 1)
}

const ptrSize = unsafe.Sizeof((*int)(nil))

const callbackMaxFrame = 64 * ptrSize

// callbackasm is implemented in zcallback_GOOS_GOARCH.s
//
//go:linkname __callbackasm callbackasm
var __callbackasm byte
var callbackasmABI0 = uintptr(unsafe.Pointer(&__callbackasm))

// callbackWrap_call allows the calling of the ABIInternal wrapper
// which is required for runtime.cgocallback without the
// <ABIInternal> tag which is only allowed in the runtime.
// This closure is used inside sys_darwin_GOARCH.s
var callbackWrap_call = callbackWrap

// callbackWrap is called by assembly code which determines which Go function to call.
// This function takes the arguments and passes them to the Go function and returns the result.
func callbackWrap(a *callbackArgs) {
	cbs.lock.Lock()
	fn := cbs.funcs[a.index]
	cbs.lock.Unlock()
	fnType := fn.Type()
	args := make([]reflect.Value, fnType.NumIn())
	frame := (*[callbackMaxFrame]uintptr)(a.args)
	// stackFrame points to stack-passed arguments. On most architectures this is
	// contiguous with frame (after register args), but on ppc64le it's separate.
	var stackFrame *[callbackMaxFrame]uintptr
	if sf := a.stackFrame(); sf != nil {
		// Only ppc64le uses separate stackArgs pointer due to NOSPLIT constraints
		stackFrame = (*[callbackMaxFrame]uintptr)(sf)
	}
	// floatsN and intsN track the number of register slots used, not argument count.
	// This distinction matters on ARM32 where float64 uses 2 slots (32-bit registers).
	var floatsN int
	var intsN int
	// stackSlot points to the index into frame (or stackFrame) of the current stack element.
	// When stackFrame is nil, stack begins after float and integer registers in frame.
	// When stackFrame is not nil (ppc64le), stackSlot indexes into stackFrame starting at 0.
	stackSlot := numOfIntegerRegisters() + numOfFloatRegisters()
	if stackFrame != nil {
		// ppc64le: stackArgs is a separate pointer, indices start at 0
		stackSlot = 0
	}
	// stackByteOffset tracks the byte offset within the stack area for Darwin ARM64
	// tight packing. On Darwin ARM64, C passes small types packed on the stack.
	stackByteOffset := uintptr(0)
	for i := range args {
		// slots is the number of pointer-sized slots the argument takes
		var slots int
		inType := fnType.In(i)
		switch inType.Kind() {
		case reflect.Float32, reflect.Float64:
			slots = int((fnType.In(i).Size() + ptrSize - 1) / ptrSize)
			if floatsN+slots > numOfFloatRegisters() {
				if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
					// Darwin ARM64: read from packed stack with proper alignment
					args[i] = callbackArgFromStack(a.args, stackSlot, &stackByteOffset, inType)
				} else if stackFrame != nil {
					// ppc64le/s390x: stack args are in separate stackFrame
					if runtime.GOARCH == "s390x" {
						// s390x big-endian: sub-8-byte values are right-justified
						args[i] = callbackArgFromSlotBigEndian(unsafe.Pointer(&stackFrame[stackSlot]), inType)
					} else {
						args[i] = reflect.NewAt(inType, unsafe.Pointer(&stackFrame[stackSlot])).Elem()
					}
					stackSlot += slots
				} else {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
					stackSlot += slots
				}
			} else {
				if runtime.GOARCH == "s390x" {
					// s390x big-endian: float32 is right-justified in 8-byte FPR slot
					args[i] = callbackArgFromSlotBigEndian(unsafe.Pointer(&frame[floatsN]), inType)
				} else {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[floatsN])).Elem()
				}
			}
			floatsN += slots
		case reflect.Struct:
			// This is the CDecl field
			args[i] = reflect.Zero(inType)
		default:
			slots = int((inType.Size() + ptrSize - 1) / ptrSize)
			if intsN+slots > numOfIntegerRegisters() {
				if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
					// Darwin ARM64: read from packed stack with proper alignment
					args[i] = callbackArgFromStack(a.args, stackSlot, &stackByteOffset, inType)
				} else if stackFrame != nil {
					// ppc64le/s390x: stack args are in separate stackFrame
					if runtime.GOARCH == "s390x" {
						// s390x big-endian: sub-8-byte values are right-justified
						args[i] = callbackArgFromSlotBigEndian(unsafe.Pointer(&stackFrame[stackSlot]), inType)
					} else {
						args[i] = reflect.NewAt(inType, unsafe.Pointer(&stackFrame[stackSlot])).Elem()
					}
					stackSlot += slots
				} else {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
					stackSlot += slots
				}
			} else {
				// the integers begin after the floats in frame
				pos := intsN + numOfFloatRegisters()
				if runtime.GOARCH == "s390x" {
					// s390x big-endian: sub-8-byte values are right-justified in GPR slot
					args[i] = callbackArgFromSlotBigEndian(unsafe.Pointer(&frame[pos]), inType)
				} else {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[pos])).Elem()
				}
			}
			intsN += slots
		}
	}
	ret := fn.Call(args)
	if len(ret) > 0 {
		switch k := ret[0].Kind(); k {
		case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uintptr:
			a.result = uintptr(ret[0].Uint())
		case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
			a.result = uintptr(ret[0].Int())
		case reflect.Bool:
			if ret[0].Bool() {
				a.result = 1
			} else {
				a.result = 0
			}
		case reflect.Pointer:
			a.result = ret[0].Pointer()
		case reflect.UnsafePointer:
			a.result = ret[0].Pointer()
		default:
			panic("purego: unsupported kind: " + k.String())
		}
	}
}

// callbackArgFromStack reads an argument from the tightly-packed stack area on Darwin ARM64.
// The C ABI on Darwin ARM64 packs small types on the stack without padding to 8 bytes.
// This function handles proper alignment and advances stackByteOffset accordingly.
func callbackArgFromStack(argsBase unsafe.Pointer, stackSlot int, stackByteOffset *uintptr, inType reflect.Type) reflect.Value {
	// Calculate base address of stack area (after float and int registers)
	stackBase := unsafe.Add(argsBase, stackSlot*int(ptrSize))

	// Get type's natural alignment
	align := uintptr(inType.Align())
	size := inType.Size()

	// Align the offset
	if *stackByteOffset%align != 0 {
		*stackByteOffset = (*stackByteOffset + align - 1) &^ (align - 1)
	}

	// Read value at aligned offset
	ptr := unsafe.Add(stackBase, *stackByteOffset)
	*stackByteOffset += size

	return reflect.NewAt(inType, ptr).Elem()
}

// callbackArgFromSlotBigEndian reads an argument from an 8-byte slot on big-endian architectures.
// On s390x:
// - Integer types are right-justified in GPRs: sub-8-byte values are at offset (8 - size)
// - Float32 in FPRs is left-justified: stored in upper 32 bits, so at offset 0
// - Float64 occupies the full 8-byte slot
func callbackArgFromSlotBigEndian(slotPtr unsafe.Pointer, inType reflect.Type) reflect.Value {
	size := inType.Size()
	if size >= 8 {
		// 8-byte values occupy the entire slot
		return reflect.NewAt(inType, slotPtr).Elem()
	}
	// Float32 is left-justified in FPRs (upper 32 bits), so offset is 0
	if inType.Kind() == reflect.Float32 {
		return reflect.NewAt(inType, slotPtr).Elem()
	}
	// Integer types are right-justified: offset = 8 - size
	offset := 8 - size
	ptr := unsafe.Add(slotPtr, offset)
	return reflect.NewAt(inType, ptr).Elem()
}

// callbackasmAddr returns address of runtime.callbackasm
// function adjusted by i.
// On x86 and amd64, runtime.callbackasm is a series of CALL instructions,
// and we want callback to arrive at
// correspondent call instruction instead of start of
// runtime.callbackasm.
// On ARM, runtime.callbackasm is a series of mov and branch instructions.
// R12 is loaded with the callback index. Each entry is two instructions,
// hence 8 bytes.
func callbackasmAddr(i int) uintptr {
	var entrySize int
	switch runtime.GOARCH {
	default:
		panic("purego: unsupported architecture")
	case "amd64":
		// On amd64, each callback entry is just a CALL instruction (5 bytes)
		entrySize = 5
	case "386":
		// On 386, each callback entry is MOVL $imm, CX (5 bytes) + JMP (5 bytes)
		entrySize = 10
	case "arm", "arm64", "loong64", "ppc64le", "riscv64":
		// On ARM, ARM64, Loong64, PPC64LE and RISCV64, each entry is a MOV instruction
		// followed by a branch instruction
		entrySize = 8
	case "s390x":
		// On S390X, each entry is LGHI (4 bytes) + JG (6 bytes)
		entrySize = 10
	}
	return callbackasmABI0 + uintptr(i*entrySize)
}
