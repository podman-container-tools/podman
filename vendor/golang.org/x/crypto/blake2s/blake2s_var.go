// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !amd64 && !386

package blake2s

var (
	useSSE4  = false
	useSSSE3 = false
	useSSE2  = false
)
