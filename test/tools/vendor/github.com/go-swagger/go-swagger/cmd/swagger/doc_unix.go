//go:build unix

// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func setTermsize(cols uint16) (restore func(), err error) {
	master, slave, err := pty.Open()
	if err != nil {
		return nil, err
	}

	const termRows uint16 = 80
	size := &pty.Winsize{Rows: termRows, Cols: cols, X: 0, Y: 0}
	err = pty.Setsize(slave, size)
	if err != nil {
		return nil, err
	}

	stdin, err := unix.Dup(syscall.Stdin)
	if err != nil {
		return nil, err
	}

	if err = unix.Dup2(int(slave.Fd()), syscall.Stdin); err != nil {
		return nil, err
	}

	return func() {
		// close pseudo-tty machinery
		_ = slave.Close()
		_ = master.Close()
		// restore stdin
		_ = unix.Dup2(stdin, syscall.Stdin)
	}, nil
}
