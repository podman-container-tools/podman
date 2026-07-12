// Copyright 2018 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !386

package vecnet

import (
	"io"
	"runtime"
	"syscall"
	"unsafe"
)

var readFromBuffers = readFromBuffersLinux

func readFromBuffersLinux(bufs Buffers, conn syscall.Conn) (int64, error) {
	rc, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}

	length := int64(0)
	for _, buf := range bufs {
		length += int64(len(buf))
	}

	for n := int64(0); n < length; {
		cur, err := recvmsg(bufs, rc)
		if err != nil && (cur == 0 || err != io.EOF) {
			return n, err
		}
		n += int64(cur)

		// Consume iovecs to retry.
		for consumed := 0; consumed < cur; {
			if len(bufs[0]) <= cur-consumed {
				consumed += len(bufs[0])
				bufs = bufs[1:]
			} else {
				bufs[0] = bufs[0][cur-consumed:]
				break
			}
		}
	}
	return length, nil
}

// buildIovec builds an iovec slice from the given []byte slice.
//
// iovecs is used as an initial slice, to avoid excessive allocations.
func buildIovec(bufs Buffers, iovecs []syscall.Iovec) ([]syscall.Iovec, int) {
	var length int
	for _, buf := range bufs {
		if l := len(buf); l > 0 {
			iovecs = append(iovecs, syscall.Iovec{
				Base: &buf[0],
				Len:  iovlen(l),
			})
			length += l
		}
	}
	return iovecs, length
}

func recvmsg(bufs Buffers, rc syscall.RawConn) (int, error) {
	iovecs, length := buildIovec(bufs, make([]syscall.Iovec, 0, 2))

	var msg syscall.Msghdr
	if len(iovecs) != 0 {
		msg.Iov = &iovecs[0]
		msg.Iovlen = iovlen(len(iovecs))
	}

	// n is the bytes received.
	var n uintptr
	var e syscall.Errno
	err := rc.Read(func(fd uintptr) bool {
		n, _, e = syscall.Syscall(syscall.SYS_RECVMSG, fd, uintptr(unsafe.Pointer(&msg)), syscall.MSG_DONTWAIT)
		// Return false if EINTR, EAGAIN, or EWOULDBLOCK to retry.
		return !(e == syscall.EINTR || e == syscall.EAGAIN || e == syscall.EWOULDBLOCK)
	})
	runtime.KeepAlive(iovecs)
	if err != nil {
		return 0, err
	}
	if e != 0 {
		return 0, e
	}

	// The other end is closed by returning a 0 length read with no error.
	if n == 0 {
		return 0, io.EOF
	}

	if int(n) > length {
		return length, io.ErrShortBuffer
	}
	return int(n), nil
}
