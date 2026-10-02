// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || freebsd || linux || netbsd || windows

package purego

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"structs"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego/internal/strings"
)

const (
	align8ByteMask = 7 // Mask for 8-byte alignment: (val + 7) &^ 7
	align8ByteSize = 8 // 8-byte alignment boundary
)

func isARMSoftFloat() bool {
	return runtime.GOARCH == "arm" && *(*uint8)(unsafe.Pointer(&runtime_goarmsoftfp)) != 0
}

var thePool = sync.Pool{New: func() any {
	return new(syscallArgs)
}}

// RegisterLibFunc is a wrapper around RegisterFunc that uses the C function returned from Dlsym(handle, name).
// It panics if it can't find the name symbol.
func RegisterLibFunc(fptr any, handle uintptr, name string) {
	sym, err := loadSymbol(handle, name)
	if err != nil {
		panic(err)
	}
	RegisterFunc(fptr, sym)
}

// RegisterFunc takes a pointer to a Go function representing the calling convention of the C function.
// fptr will be set to a function that when called will call the C function given by cfn with the
// parameters passed in the correct registers and stack.
//
// A panic is produced if the type is not a function pointer or if the function returns more than 1 value.
//
// These conversions describe how a Go type in the fptr will be used to call
// the C function. It is important to note that there is no way to verify that fptr
// matches the C function. This also holds true for struct types where the padding
// needs to be ensured to match that of C; RegisterFunc does not verify this.
//
// # Type Conversions (Go <=> C)
//
//	string <=> char*
//	bool <=> _Bool
//	uintptr <=> uintptr_t
//	uint <=> uint32_t or uint64_t
//	uint8 <=> uint8_t
//	uint16 <=> uint16_t
//	uint32 <=> uint32_t
//	uint64 <=> uint64_t
//	int <=> int32_t or int64_t
//	int8 <=> int8_t
//	int16 <=> int16_t
//	int32 <=> int32_t
//	int64 <=> int64_t
//	float32 <=> float
//	float64 <=> double
//	struct <=> struct (android, darwin, ios, linux, and windows on amd64/arm64)
//	func <=> C function
//	unsafe.Pointer, *T <=> void*
//	[]T => void*
//
// There is a special case when the last argument of fptr is a variadic interface (or []interface}
// it will be expanded into a call to the C function as if it had the arguments in that slice.
// This means that using arg ...any is like a cast to the function with the arguments inside arg.
// This is not the same as C variadic.
//
// # Memory
//
// In general it is not possible for purego to guarantee the lifetimes of objects returned or received from
// calling functions using RegisterFunc. For arguments to a C function it is important that the C function doesn't
// hold onto a reference to Go memory. This is the same as the [Cgo rules].
//
// However, there are some special cases. When passing a string as an argument if the string does not end in a null
// terminated byte (\x00) then the string will be copied into memory maintained by purego. The memory is only valid for
// that specific call. Therefore, if the C code keeps a reference to that string it may become invalid at some
// undefined time. However, if the string does already contain a null-terminated byte then no copy is done.
// It is then the responsibility of the caller to ensure the string stays alive as long as it's needed in C memory.
// This can be done using runtime.KeepAlive or allocating the string in C memory using malloc. When a C function
// returns a null-terminated pointer to char a Go string can be used. Purego will allocate a new string in Go memory
// and copy the data over. This string will be garbage collected whenever Go decides it's no longer referenced.
// This C created string will not be freed by purego. If the pointer to char is not null-terminated or must continue
// to point to C memory (because it's a buffer for example) then use a pointer to byte and then convert that to a slice
// using unsafe.Slice. Doing this means that it becomes the responsibility of the caller to care about the lifetime
// of the pointer
//
// # Structs
//
// Purego can handle the most common structs that have fields of builtin types like int8, uint16, float32, etc. However,
// it does not support aligning fields properly. It is therefore the responsibility of the caller to ensure
// that all padding is added to the Go struct to match the C one. See `BoolStructFn` in struct_test.go for an example.
//
// On Apple ARM64 platforms (macOS and iOS), purego handles proper alignment of struct arguments
// when passing them on the stack, following the C ABI's byte-level packing rules.
//
// On Windows, struct arguments and returns are supported on amd64 and arm64 when calling C functions.
// Passing or returning structs in callbacks created with [NewCallback] is not supported on Windows.
//
// # Example
//
// All functions below call this C function:
//
//	char *foo(char *str);
//
//	// Let purego convert types
//	var foo func(s string) string
//	goString := foo("copied")
//	// Go will garbage collect this string
//
//	// Manually, handle allocations
//	var foo2 func(b string) *byte
//	mustFree := foo2("not copied\x00")
//	defer free(mustFree)
//
// [Cgo rules]: https://pkg.go.dev/cmd/cgo#hdr-Go_references_to_C
func RegisterFunc(fptr any, cfn uintptr) {
	const is32bit = unsafe.Sizeof(uintptr(0)) == 4
	fn := reflect.ValueOf(fptr).Elem()
	ty := fn.Type()
	if ty.Kind() != reflect.Func {
		panic("purego: fptr must be a function pointer")
	}
	if ty.NumOut() > 1 {
		panic("purego: function can only return zero or one values")
	}
	if cfn == 0 {
		panic("purego: cfn is nil")
	}
	if ty.NumOut() == 1 && (ty.Out(0).Kind() == reflect.Float32 || ty.Out(0).Kind() == reflect.Float64) &&
		runtime.GOARCH != "arm" && runtime.GOARCH != "arm64" && runtime.GOARCH != "386" && runtime.GOARCH != "amd64" && runtime.GOARCH != "loong64" && runtime.GOARCH != "ppc64le" && runtime.GOARCH != "riscv64" && runtime.GOARCH != "s390x" {
		panic("purego: float returns are not supported")
	}
	{
		// this code checks how many registers and stack this function will use
		// to avoid crashing with too many arguments
		var ints int
		var floats int
		floatArgRegs := numOfFloatRegisters()
		ptrSize := unsafe.Sizeof(uintptr(0))
		var stack int
		for i := range ty.NumIn() {
			arg := ty.In(i)
			switch arg.Kind() {
			case reflect.Func:
				// This only does preliminary testing to ensure the CDecl argument
				// is the first argument. Full testing is done when the callback is actually
				// created in NewCallback.
				for j := range arg.NumIn() {
					in := arg.In(j)
					if !in.AssignableTo(reflect.TypeFor[CDecl]()) {
						continue
					}
					if j != 0 {
						panic("purego: CDecl must be the first argument")
					}
				}
			case reflect.String, reflect.Uintptr, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Pointer, reflect.UnsafePointer,
				reflect.Slice, reflect.Bool:
				if ints < numOfIntegerRegisters() {
					ints++
				} else {
					stack++
				}
			case reflect.Uint64, reflect.Int64:
				usesSlots := max(1, int(arg.Size()/ptrSize))
				if isARMPaddingNeeded(arg, ints, stack) {
					usesSlots++
				}

				if ints < numOfIntegerRegisters() {
					ints += usesSlots
				} else {
					stack += usesSlots
				}
			case reflect.Float32, reflect.Float64:
				usesSlots := max(1, int(arg.Size()/ptrSize))
				if isARMSoftFloat() {
					// float64 for arm with softfloat uses same rules as int64
					if isARMPaddingNeeded(arg, ints, stack) {
						usesSlots++
					}
					if ints < numOfIntegerRegisters() {
						ints += usesSlots
					} else {
						stack += usesSlots
					}
					continue
				}

				if isARMFloatPaddingNeeded(arg, floats, stack) {
					usesSlots++
				}
				if floats < floatArgRegs {
					floats++
				} else {
					stack += usesSlots
				}
			case reflect.Struct:
				ensureStructSupported()
				if arg.Size() == 0 && runtime.GOOS != "windows" {
					// On Windows an empty struct still consumes one argument slot.
					continue
				}
				addInt := func(u uintptr) {
					ints++
				}
				addFloat := func(u uintptr) {
					floats++
				}
				addStack := func(u uintptr) {
					stack++
				}
				_ = addStruct(reflect.New(arg).Elem(), &ints, &floats, &stack, addInt, addFloat, addStack, nil)
			default:
				panic("purego: unsupported kind " + arg.Kind().String())
			}
		}
		if ty.NumOut() == 1 && ty.Out(0).Kind() == reflect.Struct {
			ensureStructSupported()
			outType := ty.Out(0)
			checkStructFieldsSupported(outType)
			if structReturnInMemory(outType) {
				// A struct returned in memory is allocated by the caller and its
				// pointer is passed as a hidden first integer argument. When the
				// integer registers are already full, prepending it spills a
				// regular argument onto the stack.
				if ints < numOfIntegerRegisters() {
					ints++
				} else {
					stack++
				}
			}
		}

		argsLimit := maxArgs
		sizeOfStack := argsLimit - numOfIntegerRegisters()
		if runtime.GOOS == "windows" {
			if ints+floats+stack > argsLimit {
				panic("purego: too many stack arguments")
			}
		} else if isDarwin && runtime.GOARCH == "arm64" {
			// On Darwin ARM64, use byte-based validation since arguments pack efficiently.
			// See https://developer.apple.com/documentation/xcode/writing-arm64-code-for-apple-platforms
			stackBytes := estimateStackBytes(ty)
			maxStackBytes := sizeOfStack * 8
			if stackBytes > maxStackBytes {
				panic("purego: too many stack arguments")
			}
		} else {
			if stack > sizeOfStack {
				panic("purego: too many stack arguments")
			}
		}
	}

	v := reflect.MakeFunc(ty, func(args []reflect.Value) (results []reflect.Value) {
		var sysargs [maxArgs]uintptr
		// Use maxArgs instead of numOfFloatRegisters() to keep this code path allocation-free,
		// since numOfFloatRegisters() is a function call, not a constant.
		// maxArgs is always greater than or equal to numOfFloatRegisters() so this is safe.
		var floats [maxArgs]uintptr
		floatArgRegs := numOfFloatRegisters()
		var numInts int
		var numFloats int
		var numStack int
		var addStack, addInt, addFloat func(x uintptr)
		if runtime.GOARCH == "arm64" || runtime.GOOS != "windows" {
			// Windows arm64 uses the same calling convention as macOS and Linux
			addStack = func(x uintptr) {
				sysargs[numOfIntegerRegisters()+numStack] = x
				numStack++
			}
			addInt = func(x uintptr) {
				if numInts >= numOfIntegerRegisters() {
					addStack(x)
				} else {
					sysargs[numInts] = x
					numInts++
				}
			}
			addFloat = func(x uintptr) {
				if numFloats < floatArgRegs {
					floats[numFloats] = x
					numFloats++
				} else {
					addStack(x)
				}
			}
		} else {
			// On Windows amd64 the arguments are passed in the numbered registered.
			// So the first int is in the first integer register and the first float
			// is in the second floating register if there is already a first int.
			// This is in contrast to how macOS and Linux pass arguments which
			// tries to use as many registers as possible in the calling convention.
			addStack = func(x uintptr) {
				if numStack >= maxArgs {
					panic("purego: too many stack arguments")
				}
				sysargs[numStack] = x
				numStack++
			}
			addInt = addStack
			addFloat = addStack
		}

		var keepAlive []any
		defer func() {
			runtime.KeepAlive(keepAlive)
			runtime.KeepAlive(args)
		}()

		var arm64_r8 uintptr
		if ty.NumOut() == 1 && ty.Out(0).Kind() == reflect.Struct {
			outType := ty.Out(0)
			if structReturnInMemory(outType) {
				// The caller allocates the return value and passes its pointer
				// as a hidden first integer argument.
				val := reflect.New(outType)
				keepAlive = append(keepAlive, val)
				addInt(val.Pointer())
			} else if runtime.GOARCH == "arm64" && outType.Size() > maxRegAllocStructSize {
				isAllFloats, numFields := isAllSameFloat(outType)
				if !isAllFloats || numFields > 4 {
					val := reflect.New(outType)
					keepAlive = append(keepAlive, val)
					arm64_r8 = val.Pointer()
				}
			}
		}
		for i, v := range args {
			if variadic, ok := reflect.TypeAssert[[]any](args[i]); ok {
				if i != len(args)-1 {
					panic("purego: can only expand last parameter")
				}
				for _, x := range variadic {
					keepAlive = addValue(reflect.ValueOf(x), keepAlive, addInt, addFloat, addStack, &numInts, &numFloats, &numStack)
				}
				continue
			}
			// Check if we need to start Darwin ARM64 C-style stack packing
			if runtime.GOARCH == "arm64" && isDarwin && shouldBundleStackArgs(v, numInts, numFloats) {
				// Collect and separate remaining args into register vs stack
				stackArgs, newKeepAlive := collectStackArgs(args, i, numInts, numFloats,
					keepAlive, addInt, addFloat, addStack, &numInts, &numFloats, &numStack)
				keepAlive = newKeepAlive

				// Bundle stack arguments with C-style packing
				bundleStackArgs(stackArgs, addStack)
				break
			}
			keepAlive = addValue(v, keepAlive, addInt, addFloat, addStack, &numInts, &numFloats, &numStack)
		}

		var syscall *syscallArgs
		if runtime.GOOS == "windows" && runtime.GOARCH != "arm64" {
			// Windows amd64, 386, and arm use syscall.SyscallN.
			syscall = thePool.Get().(*syscallArgs)
			syscall.a1, syscall.a2, _ = syscall_syscallN(cfn, sysargs[:numStack]...)
			syscall.f1 = syscall.a2 // on amd64 a2 stores the float return. On 32bit platforms floats aren't support
		} else {
			syscall = syscall_SyscallN(cfn, sysargs[:], floats[:], arm64_r8)
		}
		defer thePool.Put(syscall)
		if ty.NumOut() == 0 {
			return nil
		}
		outType := ty.Out(0)
		v := reflect.New(outType).Elem()
		switch outType.Kind() {
		case reflect.Uint64:
			if is32bit {
				// high-word is recorded at a2 for 32-bit platforms and 64-bit returns
				v.SetUint(uint64(syscall.a1) | (uint64(syscall.a2) << 32))
			} else {
				v.SetUint(uint64(syscall.a1))
			}
		case reflect.Uintptr, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
			v.SetUint(uint64(syscall.a1))
		case reflect.Int64:
			if is32bit {
				v.SetInt(int64(syscall.a1) | (int64(syscall.a2) << 32))
			} else {
				v.SetInt(int64(syscall.a1))
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
			v.SetInt(int64(syscall.a1))
		case reflect.Bool:
			v.SetBool(byte(syscall.a1) != 0)
		case reflect.UnsafePointer:
			// We take the address and then dereference it to trick go vet from creating a possible miss-use of unsafe.Pointer
			v.SetPointer(*(*unsafe.Pointer)(unsafe.Pointer(&syscall.a1)))
		case reflect.Pointer:
			// Copy syscall.a1 into a local variable to prevent v
			// from holding a pointer to the pooled syscallArgs field.
			a1 := syscall.a1
			v = reflect.NewAt(outType, unsafe.Pointer(&a1)).Elem()
		case reflect.Func:
			// wrap this C function in a nicely typed Go function
			if syscall.a1 != 0 {
				RegisterFunc(v.Addr().Interface(), syscall.a1)
			}
		case reflect.String:
			v.SetString(strings.GoString(syscall.a1))
		case reflect.Float32:
			// NOTE: syscall.r2 is only the floating return value on 64bit platforms.
			// On 32bit platforms syscall.r2 is the upper part of a 64bit return.
			// On 386, x87 FPU returns floats as float64 in ST(0), so we read as float64 and convert.
			// On PPC64LE, C ABI converts float32 to double in FPR, so we read as float64.
			// On S390X (big-endian), float32 is in upper 32 bits of the 64-bit FP register.
			// On 32bit ARM with softfloat float32 returned as integer
			switch runtime.GOARCH {
			case "386":
				v.SetFloat(math.Float64frombits(uint64(syscall.f1) | (uint64(syscall.f2) << 32)))
			case "ppc64le":
				v.SetFloat(math.Float64frombits(uint64(syscall.f1)))
			case "s390x":
				// S390X is big-endian: float32 in upper 32 bits of 64-bit register
				v.SetFloat(float64(math.Float32frombits(uint32(syscall.f1 >> 32))))
			case "arm":
				if isARMSoftFloat() {
					v.SetFloat(float64(math.Float32frombits(uint32(syscall.a1))))
				} else {
					v.SetFloat(float64(math.Float32frombits(uint32(syscall.f1))))
				}
			default:
				v.SetFloat(float64(math.Float32frombits(uint32(syscall.f1))))
			}
		case reflect.Float64:
			// NOTE: syscall.r2 is only the floating return value on 64bit platforms.
			// On 32bit platforms syscall.r2 is the upper part of a 64bit return.
			if isARMSoftFloat() {
				// a1,a2 are populated in this case
				v.SetFloat(math.Float64frombits(uint64(syscall.a1) | (uint64(syscall.a2) << 32)))
			} else if is32bit {
				v.SetFloat(math.Float64frombits(uint64(syscall.f1) | (uint64(syscall.f2) << 32)))
			} else {
				v.SetFloat(math.Float64frombits(uint64(syscall.f1)))
			}
		case reflect.Struct:
			v = getStruct(outType, *syscall)
		default:
			panic("purego: unsupported return kind: " + outType.Kind().String())
		}
		if len(args) > 0 {
			// reuse args slice instead of allocating one when possible
			args[0] = v
			return args[:1]
		} else {
			return []reflect.Value{v}
		}
	})
	fn.Set(v)
}

func addValue(v reflect.Value, keepAlive []any, addInt func(x uintptr), addFloat func(x uintptr), addStack func(x uintptr), numInts *int, numFloats *int, numStack *int) []any {
	const is32bit = unsafe.Sizeof(uintptr(0)) == 4
	switch v.Kind() {
	case reflect.String:
		ptr := strings.CString(v.String())
		keepAlive = append(keepAlive, ptr)
		addInt(uintptr(unsafe.Pointer(ptr)))
	case reflect.Uint64:
		if isARMPaddingNeeded(v.Type(), *numInts, *numStack) {
			addInt(0)
		}
		addInt(uintptr(v.Uint()))
		if is32bit {
			addInt(uintptr(v.Uint() >> 32)) // on 32bit we must add high word too
		}
	case reflect.Uintptr, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		addInt(uintptr(v.Uint()))
	case reflect.Int64:
		if isARMPaddingNeeded(v.Type(), *numInts, *numStack) {
			addInt(0)
		}
		addInt(uintptr(v.Int()))
		if is32bit {
			addInt(uintptr(v.Int() >> 32))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		addInt(uintptr(v.Int()))
	case reflect.Pointer, reflect.UnsafePointer, reflect.Slice:
		// There is no need to keepAlive this pointer separately because it is kept alive in the args variable
		addInt(v.Pointer())
	case reflect.Func:
		addInt(NewCallback(v.Interface()))
	case reflect.Bool:
		if v.Bool() {
			addInt(1)
		} else {
			addInt(0)
		}
	case reflect.Float32:
		switch runtime.GOARCH {
		case "ppc64le":
			// A single-precision argument occupies a floating-point register in double format on Power.
			addFloat(uintptr(math.Float64bits(v.Float())))
		case "s390x":
			// S390X big-endian: float32 goes in the upper 32 bits of the 64-bit FP register.
			addFloat(uintptr(math.Float32bits(float32(v.Float()))) << 32)
		case "arm":
			if isARMSoftFloat() {
				// 32-bit ARM with softfloat: float32 goes as integer
				addInt(uintptr(math.Float32bits(float32(v.Float()))))
			} else {
				addFloat(uintptr(math.Float32bits(float32(v.Float()))))
			}
		default:
			addFloat(uintptr(math.Float32bits(float32(v.Float()))))
		}
	case reflect.Float64:
		bits := math.Float64bits(v.Float())
		if isARMFloatPaddingNeeded(v.Type(), *numFloats, *numStack) {
			// if floats are spilled onto stack on ARM than we must follow AAPCS C.7
			addFloat(0)
		}
		if isARMSoftFloat() {
			// add as uint64
			if isARMPaddingNeeded(v.Type(), *numInts, *numStack) {
				addInt(0)
			}
			addInt(uintptr(bits))
			addInt(uintptr(bits >> 32))
		} else if is32bit {
			addFloat(uintptr(bits))
			addFloat(uintptr(bits >> 32))
		} else {
			addFloat(uintptr(bits))
		}
	case reflect.Struct:
		keepAlive = addStruct(v, numInts, numFloats, numStack, addInt, addFloat, addStack, keepAlive)
	default:
		panic("purego: unsupported kind: " + v.Kind().String())
	}
	return keepAlive
}

// maxRegAllocStructSize is the biggest a struct can be while still fitting in registers.
// if it is bigger than this than enough space must be allocated on the heap and then passed into
// the function as the first parameter on amd64 or in R8 on arm64.
//
// If you change this make sure to update it in objc_runtime_darwin.go
const maxRegAllocStructSize = 16

var hostLayoutType = reflect.TypeFor[structs.HostLayout]()

// isABIField reports whether f takes part in the C ABI of the struct that
// contains it. Only the structs.HostLayout marker does not.
func isABIField(f reflect.StructField) bool {
	return !f.Type.ConvertibleTo(hostLayoutType)
}

// numABIFields returns how many of ty's fields take part in the C ABI.
func numABIFields(ty reflect.Type) int {
	var n int
	for i := range ty.NumField() {
		if isABIField(ty.Field(i)) {
			n++
		}
	}
	return n
}

// abiField returns the i'th field of ty that takes part in the C ABI. It panics
// if ty has fewer than i+1 such fields.
func abiField(ty reflect.Type, i int) reflect.StructField {
	for j := range ty.NumField() {
		f := ty.Field(j)
		if !isABIField(f) {
			continue
		}
		if i == 0 {
			return f
		}
		i--
	}
	panic("purego: struct field index out of range")
}

func isAllSameFloat(ty reflect.Type) (allFloats bool, numFields int) {
	allFloats = true
	if numABIFields(ty) == 0 {
		return false, 0
	}
	root := abiField(ty, 0).Type
	for root.Kind() == reflect.Struct {
		if numABIFields(root) == 0 {
			return false, 0
		}
		root = abiField(root, 0).Type
	}
	first := root.Kind()
	if first != reflect.Float32 && first != reflect.Float64 {
		allFloats = false
	}
	for i := range ty.NumField() {
		if !isABIField(ty.Field(i)) {
			continue
		}
		f := ty.Field(i).Type
		if f.Kind() == reflect.Struct {
			var structNumFields int
			allFloats, structNumFields = isAllSameFloat(f)
			numFields += structNumFields
			continue
		}
		numFields++
		if f.Kind() != first {
			allFloats = false
		}
	}
	return allFloats, numFields
}

func checkStructFieldsSupported(ty reflect.Type) {
	for i := range ty.NumField() {
		if !isABIField(ty.Field(i)) {
			continue
		}
		f := ty.Field(i).Type
		if f.Kind() == reflect.Array {
			f = f.Elem()
		} else if f.Kind() == reflect.Struct {
			checkStructFieldsSupported(f)
			continue
		}
		switch f.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Uintptr, reflect.Pointer, reflect.UnsafePointer, reflect.Float64, reflect.Float32,
			reflect.Bool:
		default:
			panic(fmt.Sprintf("purego: struct field type %s is not supported", f))
		}
	}
}

// ensureStructSupported panics if passing or returning structs through a call to
// a C function is unsupported on the current platform.
func ensureStructSupported() {
	switch runtime.GOARCH {
	case "amd64", "arm64", "loong64", "ppc64le":
	default:
		panic("purego: struct arguments/returns are only supported on amd64, arm64, loong64, and ppc64le")
	}
	switch runtime.GOOS {
	case "android", "darwin", "ios", "linux", "windows":
	default:
		panic("purego: struct arguments/returns are only supported on android, darwin, ios, linux, and windows")
	}
}

// ensureCallbackStructSupported panics if passing or returning structs through a
// callback is unsupported on the current platform. Callbacks support structs on
// fewer architectures than a direct call to a C function.
func ensureCallbackStructSupported() {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		panic("purego: struct arguments/returns in callbacks are only supported on amd64 and arm64")
	}
	switch runtime.GOOS {
	case "android", "darwin", "ios", "linux", "windows":
	default:
		panic("purego: struct arguments/returns in callbacks are only supported on android, darwin, ios, linux, and windows")
	}
}

// isDarwin is true on platforms that use Apple's calling convention.
// iOS (GOOS=ios) shares it with macOS (GOOS=darwin).
const isDarwin = runtime.GOOS == "darwin" || runtime.GOOS == "ios"

func roundUpTo8(val uintptr) uintptr {
	return (val + align8ByteMask) &^ align8ByteMask
}

func numOfFloatRegisters() int {
	switch runtime.GOARCH {
	case "amd64", "arm64", "loong64", "ppc64le", "riscv64":
		return 8
	case "s390x":
		return 4
	case "arm":
		// 8 doubles (16 words) are always reserved by asm trampolines, even if softfloat is used
		return 16
	case "386":
		// i386 SysV ABI passes all arguments on the stack, including floats
		return 0
	default:
		// since this platform isn't supported and can therefore only access
		// integer registers it is safest to return 8
		return 8
	}
}

func numOfIntegerRegisters() int {
	switch runtime.GOARCH {
	case "arm64", "loong64", "ppc64le", "riscv64":
		return 8
	case "amd64":
		return 6
	case "s390x":
		// S390X uses R2-R6 for integer arguments
		return 5
	case "arm":
		return 4
	case "386":
		// i386 SysV ABI passes all arguments on the stack
		return 0
	default:
		// since this platform isn't supported and can therefore only access
		// integer registers it is fine to return the maxArgs
		return maxArgs
	}
}

// estimateStackBytes estimates stack bytes needed for Darwin ARM64 validation.
// This is a conservative estimate used only for early error detection.
func estimateStackBytes(ty reflect.Type) int {
	var numInts, numFloats int
	var stackBytes int

	for i := range ty.NumIn() {
		arg := ty.In(i)
		size := int(arg.Size())

		// Check if this goes to register or stack
		usesInt := arg.Kind() != reflect.Float32 && arg.Kind() != reflect.Float64
		if usesInt && numInts < numOfIntegerRegisters() {
			numInts++
		} else if !usesInt && numFloats < numOfFloatRegisters() {
			numFloats++
		} else {
			stackBytes += size
		}
	}
	// Round total to 8-byte boundary
	if stackBytes > 0 && stackBytes%align8ByteSize != 0 {
		stackBytes = int(roundUpTo8(uintptr(stackBytes)))
	}
	return stackBytes
}

func isARMPaddingNeeded(ty reflect.Type, numInts, numStack int) bool {
	// ARM EABI (AAPCS): 8-byte-aligned types (int64/uint64) start on an
	// even core register (C.3); if they then spill, the stack slot is
	// 8-byte aligned too (C.7).
	// https://github.com/ARM-software/abi-aa/blob/main/aapcs32/aapcs32.rst#6111handling-values-larger-than-32-bits
	if runtime.GOARCH != "arm" || ty.Size() != 8 {
		return false
	}
	if numInts >= 0 && numInts < numOfIntegerRegisters() {
		return numInts%2 != 0
	}
	return numStack%2 != 0
}

func isARMFloatPaddingNeeded(ty reflect.Type, numFloats, numStack int) bool {
	if runtime.GOARCH != "arm" || ty.Size() != 8 {
		return false
	}
	if numFloats >= 0 && numFloats < numOfFloatRegisters() {
		// float registers are 64bit so alignment never needed for args in registers
		return false
	}
	// Only check if AAPCS C.7 is applicable here
	return numStack%2 != 0
}
