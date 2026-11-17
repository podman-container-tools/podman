//go:build !unix

// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

func setTermsize(cols uint16) (restore func(), err error) {
	return func() {}, nil // noop on windows
}
