// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux && (ppc64le || s390x)

package purego

import "unsafe"

type callbackArgs struct {
	index uintptr
	// args points to the argument block.
	//
	// The structure of the arguments goes
	// float registers followed by the
	// integer registers followed by the stack.
	//
	// This variable is treated as a continuous
	// block of memory containing all of the arguments
	// for this callback.
	args unsafe.Pointer
	// Below are out-args from callbackWrap
	result uintptr
	// stackArgs points to stack-passed arguments for architectures where
	// they can't be made contiguous with register args (e.g., ppc64le).
	// On other architectures, this is nil and stack args are read from
	// the end of the args block.
	stackArgs unsafe.Pointer
}

func (c *callbackArgs) stackFrame() unsafe.Pointer {
	return c.stackArgs
}
