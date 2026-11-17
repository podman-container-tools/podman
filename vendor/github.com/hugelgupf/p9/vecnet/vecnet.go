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

// Package vecnet provides access to recvmsg syscalls on net.Conns.
package vecnet

import (
	"io"
	"net"
	"syscall"
)

// Buffers points to zero or more buffers to read into.
//
// On connections that support it, ReadFrom is optimized into the batch read
// operation recvmsg.
type Buffers net.Buffers

// ReadFrom reads into the pre-allocated bufs. Returns bytes read.
//
// ReadFrom keeps reading until all bufs are filled or EOF is received.
//
// The pre-allocatted space used by ReadFrom is based upon slice lengths.
func (bufs Buffers) ReadFrom(r io.Reader) (int64, error) {
	if conn, ok := r.(syscall.Conn); ok && readFromBuffers != nil {
		return readFromBuffers(bufs, conn)
	}

	var total int64
	for _, buf := range bufs {
		for filled := 0; filled < len(buf); {
			n, err := r.Read(buf[filled:])
			total += int64(n)
			filled += n
			if (n == 0 && err == nil) || err == io.EOF {
				return total, io.EOF
			} else if err != nil {
				return total, err
			}
		}
	}
	return total, nil
}
