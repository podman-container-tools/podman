// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

package purego

import "unsafe"

// callbackArgs is the argument block passed from the assembly trampoline
// to callbackWrap when C code calls a Go callback registered with NewCallback.
type callbackArgs struct {
	index uintptr
	// args points to the argument block.
	//
	// The structure of the arguments goes
	// float registers followed by the
	// integer registers followed by the stack.
	args unsafe.Pointer
	// Below are out-args from callbackWrap.
	result [1]uintptr
	// stackArgs points to stack-passed arguments.
	stackArgs unsafe.Pointer
}

func (c *callbackArgs) stackFrame() unsafe.Pointer {
	return c.stackArgs
}

func (c *callbackArgs) intFrame() unsafe.Pointer {
	return nil
}

func (c *callbackArgs) setInt64Result(result int64) {
	c.result[0] = uintptr(result)
}

func (c *callbackArgs) setUint64Result(result uint64) {
	c.result[0] = uintptr(result)
}
