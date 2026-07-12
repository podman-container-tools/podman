// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build darwin || freebsd || (linux && (386 || amd64 || arm || arm64 || loong64 || riscv64)) || netbsd

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
}

func (c *callbackArgs) stackFrame() unsafe.Pointer {
	return nil
}
