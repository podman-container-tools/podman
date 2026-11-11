// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build loong64 && gc && !purego

package blake2s

import "golang.org/x/sys/cpu"

//go:noescape
func hashBlocksVX(h *[8]uint32, c *[2]uint32, flag uint32, blocks []byte)

func hashBlocks(h *[8]uint32, c *[2]uint32, flag uint32, blocks []byte) {
	if cpu.Loong64.HasLSX {
		hashBlocksVX(h, c, flag, blocks)
	} else {
		hashBlocksGeneric(h, c, flag, blocks)
	}
}
