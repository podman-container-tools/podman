// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || freebsd || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || (s390x && (cgo || go1.27)))) || netbsd

package purego

import (
	"math"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

var syscallXABI0 uintptr

func syscall_syscallN(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	panic("purego: syscall_syscallN is only supported on windows")
}

// NewCallback converts a Go function to a function pointer conforming to the C calling convention.
// This is useful when interoperating with C code requiring callbacks. The argument is expected to be a
// function with zero or one uintptr-sized result. The function must not have arguments with size larger than the size
// of uintptr. Only a limited number of callbacks may be created in a single Go process, and any memory allocated
// for these callbacks is never released. At least 2000 callbacks can always be created. Although this function
// provides similar functionality to windows.NewCallback it is distinct.
func NewCallback(fn any) uintptr {
	ty := reflect.TypeOf(fn)
	for i := range ty.NumIn() {
		in := ty.In(i)
		if !in.AssignableTo(reflect.TypeFor[CDecl]()) {
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
	for i := range ty.NumIn() {
		in := ty.In(i)
		switch in.Kind() {
		case reflect.Struct:
			if i == 0 && in.AssignableTo(reflect.TypeFor[CDecl]()) {
				continue
			}
			ensureCallbackStructSupported()
			checkStructFieldsSupported(in)
			continue
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
		case reflect.Struct:
			ensureCallbackStructSupported()
			checkStructFieldsSupported(ty.Out(0))
			break output
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
	var intFrame *[callbackMaxFrame]uintptr
	if sf := a.stackFrame(); sf != nil {
		// Only ppc64le uses separate stackArgs pointer due to NOSPLIT constraints
		stackFrame = (*[callbackMaxFrame]uintptr)(sf)
	}
	if intf := a.intFrame(); intf != nil {
		intFrame = (*[callbackMaxFrame]uintptr)(intf)
	}
	// floatsN and intsN track the number of register slots used, not argument count.
	// This distinction matters on ARM32 where float64 uses 2 slots (32-bit registers).
	var floatsN int
	var intsN int
	// On amd64/loong64/ppc64le/riscv64/s390x, when returning a struct larger than
	// maxRegAllocStructSize, the caller passes a hidden pointer in the first integer
	// register. Skip it to avoid misreading it as the first function argument.
	if (runtime.GOARCH == "amd64" || runtime.GOARCH == "loong64" || runtime.GOARCH == "riscv64" || runtime.GOARCH == "s390x") &&
		fnType.NumOut() == 1 && fnType.Out(0).Kind() == reflect.Struct &&
		fnType.Out(0).Size() > maxRegAllocStructSize {
		intsN = 1
	}
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
		inType := fnType.In(i)
		slots := int((inType.Size() + ptrSize - 1) / ptrSize)
		switch inType.Kind() {
		case reflect.Float32, reflect.Float64:
			if isARMSoftFloat() {
				// we should restore from integer slot, can skip unnecessary branching here
				if isARMPaddingNeeded(inType, -1, intsN) {
					intsN++
				}
				if intsN+slots <= numOfIntegerRegisters() {
					// the integers begin after the floats in frame
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[intsN+numOfFloatRegisters()])).Elem()
					intsN += slots
					continue
				}
				if isARMPaddingNeeded(inType, -1, stackSlot) {
					stackSlot++
				}
				args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
				stackSlot += slots
				intsN += slots
				continue
			}

			if floatsN+slots > numOfFloatRegisters() {
				if isDarwin && runtime.GOARCH == "arm64" {
					// Darwin ARM64: read from packed stack with proper alignment
					args[i] = callbackArgFromStack(a.args, stackSlot, &stackByteOffset, inType)
				} else if stackFrame != nil {
					// ppc64le/s390x: stack args are in separate stackFrame
					switch runtime.GOARCH {
					case "ppc64le":
						args[i] = callbackFloatFromDoubleSlot(unsafe.Pointer(&stackFrame[stackSlot]), inType)
					case "s390x":
						// s390x big-endian: sub-8-byte values are right-justified
						args[i] = callbackArgFromSlotBigEndian(unsafe.Pointer(&stackFrame[stackSlot]), inType)
					default:
						args[i] = reflect.NewAt(inType, unsafe.Pointer(&stackFrame[stackSlot])).Elem()
					}
					stackSlot += slots
				} else if isARMFloatPaddingNeeded(inType, -1, stackSlot) {
					stackSlot++
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
					stackSlot += slots
				} else {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
					stackSlot += slots
				}
			} else {
				switch runtime.GOARCH {
				case "ppc64le":
					args[i] = callbackFloatFromDoubleSlot(unsafe.Pointer(&frame[floatsN]), inType)
				case "s390x":
					// s390x big-endian: float32 is right-justified in 8-byte FPR slot
					args[i] = callbackArgFromSlotBigEndian(unsafe.Pointer(&frame[floatsN]), inType)
				default:
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[floatsN])).Elem()
				}
			}
			floatsN += slots
		case reflect.Struct:
			if i == 0 && inType.AssignableTo(reflect.TypeFor[CDecl]()) {
				args[i] = reflect.Zero(inType)
				continue
			}
			if inType.Size() == 0 {
				args[i] = reflect.New(inType).Elem()
				continue
			}
			args[i] = getCallbackStruct(inType, a.args, &floatsN, &intsN, &stackSlot, &stackByteOffset)
			continue
		default:
			if isARMPaddingNeeded(inType, -1, intsN) {
				intsN++
			}
			if intsN+slots > numOfIntegerRegisters() {
				if isDarwin && runtime.GOARCH == "arm64" {
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
				} else if isARMPaddingNeeded(inType, -1, stackSlot) {
					stackSlot++
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
					stackSlot += slots
				} else {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&frame[stackSlot])).Elem()
					stackSlot += slots
				}
			} else {
				if intFrame != nil {
					args[i] = reflect.NewAt(inType, unsafe.Pointer(&intFrame[intsN])).Elem()
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
			}
			intsN += slots
		}
	}
	ret := fn.Call(args)
	if len(ret) > 0 {
		switch k := ret[0].Kind(); k {
		case reflect.Uint64:
			a.setUint64Result(ret[0].Uint())
		case reflect.Uint, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uintptr:
			a.result[0] = uintptr(ret[0].Uint())
		case reflect.Int64:
			a.setInt64Result(ret[0].Int())
		case reflect.Int, reflect.Int32, reflect.Int16, reflect.Int8:
			a.result[0] = uintptr(ret[0].Int())
		case reflect.Bool:
			if ret[0].Bool() {
				a.result[0] = 1
			} else {
				a.result[0] = 0
			}
		case reflect.Pointer:
			a.result[0] = ret[0].Pointer()
		case reflect.UnsafePointer:
			a.result[0] = ret[0].Pointer()
		case reflect.Struct:
			setStruct(a, ret[0])
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

// callbackFloatFromDoubleSlot reads a floating-point callback argument from an
// 8-byte register or stack slot on ppc64le, where a single-precision value is
// held in double-precision format.
func callbackFloatFromDoubleSlot(slotPtr unsafe.Pointer, inType reflect.Type) reflect.Value {
	if inType.Kind() != reflect.Float32 {
		return reflect.NewAt(inType, slotPtr).Elem()
	}
	v := reflect.New(inType).Elem()
	v.SetFloat(math.Float64frombits(*(*uint64)(slotPtr)))
	return v
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
