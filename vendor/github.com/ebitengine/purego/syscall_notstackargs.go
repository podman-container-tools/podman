// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build !(ppc64le || s390x)

package purego

import "unsafe"

// callbackArgs is the argument block passed from the assembly trampoline
// to callbackWrap when C code calls a Go callback registered with NewCallback.
// The assembly fills in the fields before calling callbackWrap, which uses
// them to determine which Go function to invoke and where to read its
// arguments from, and writes the return value back into result.
//
// callbackArgs is only used on Unix. On Windows, callbacks are handled by
// the runtime's own callback mechanism, so this type is compiled but unused,
// serving only as a stub to satisfy cross-platform compilation.
type callbackArgs struct {
	index uintptr
	// args points to the argument block.
	//
	// The structure of the arguments goes
	// float registers followed by the
	// integer registers followed by the stack.
	//
	// This variable is treated as a contiguous
	// block of memory containing all of the arguments
	// for this callback.
	args unsafe.Pointer
	// Below are out-args from callbackWrap
	result [4]uintptr
}

func (c *callbackArgs) stackFrame() unsafe.Pointer {
	return nil
}

func (c *callbackArgs) intFrame() unsafe.Pointer {
	return nil
}

func (c *callbackArgs) setInt64Result(result int64) {
	c.result[0] = uintptr(result)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		c.result[1] = uintptr(result >> 32)
	}
}

func (c *callbackArgs) setUint64Result(result uint64) {
	c.result[0] = uintptr(result)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		c.result[1] = uintptr(result >> 32)
	}
}
