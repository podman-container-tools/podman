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

package p9

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"

	"github.com/hugelgupf/p9/vecnet"
	"github.com/u-root/uio/ulog"
)

// ConnError is returned in cases of a connection issue.
//
// This may be treated differently than other errors.
type ConnError struct {
	// error is the socket error.
	error
}

func (e ConnError) Error() string {
	return fmt.Sprintf("socket error: %v", e.error)
}

// Is reports whether any error in err's chain matches target.
func (e ConnError) Is(target error) bool { return target == e.error }

// ErrMessageTooLarge indicates the size was larger than reasonable.
type ErrMessageTooLarge struct {
	size  uint32
	msize uint32
}

// Error returns a sensible error.
func (e *ErrMessageTooLarge) Error() string {
	return fmt.Sprintf("message too large for fixed buffer: size is %d, limit is %d", e.size, e.msize)
}

// ErrNoValidMessage indicates no valid message could be decoded.
var ErrNoValidMessage = errors.New("buffer contained no valid message")

const (
	// headerLength is the number of bytes required for a header.
	headerLength uint32 = 7

	// maximumLength is the largest possible message.
	maximumLength uint32 = 4 * 1024 * 1024

	// initialBufferLength is the initial data buffer we allocate.
	initialBufferLength uint32 = 64
)

var dataPool = sync.Pool{
	New: func() interface{} {
		// These buffers are used for decoding without a payload.
		// We need to return a pointer to avoid unnecessary allocations
		// (see https://staticcheck.io/docs/checks#SA6002).
		b := make([]byte, initialBufferLength)
		return &b
	},
}

// send sends the given message over the socket.
func send(l ulog.Logger, w io.Writer, tag tag, m message) error {
	data := dataPool.Get().(*[]byte)
	dataBuf := buffer{data: (*data)[:0]}

	// Encode the message. The buffer will grow automatically.
	m.encode(&dataBuf)

	l.Printf("send [w %p] [Tag %06d] %s", w, tag, m)

	// Get our vectors to send.
	var hdr [headerLength]byte
	vecs := make(net.Buffers, 0, 3)
	vecs = append(vecs, hdr[:])
	if len(dataBuf.data) > 0 {
		vecs = append(vecs, dataBuf.data)
	}
	totalLength := headerLength + uint32(len(dataBuf.data))

	// Is there a payload?
	if payloader, ok := m.(payloader); ok {
		p := payloader.Payload()
		if len(p) > 0 {
			vecs = append(vecs, p)
			totalLength += uint32(len(p))
		}
		defer payloader.PayloadCleanup()
	}

	// Construct the header.
	headerBuf := buffer{data: hdr[:0]}
	headerBuf.Write32(totalLength)
	headerBuf.WriteMsgType(m.typ())
	headerBuf.WriteTag(tag)

	if _, err := vecs.WriteTo(w); err != nil {
		return ConnError{err}
	}

	// All set.
	dataPool.Put(&dataBuf.data)
	return nil
}

// lookupTagAndType looks up an existing message or creates a new one.
//
// This is called by recv after decoding the header. Any error returned will be
// propagating back to the caller. You may use messageByType directly as a
// lookupTagAndType function (by design).
type lookupTagAndType func(tag tag, t msgType) (message, error)

// recv decodes a message from the socket.
//
// This is done in two parts, and is thus not safe for multiple callers.
//
// On a socket error, the special error type ErrSocket is returned.
//
// The tag value NoTag will always be returned if err is non-nil.
func recv(l ulog.Logger, r io.Reader, msize uint32, lookup lookupTagAndType) (tag, message, error) {
	// Read a header.
	var hdr [headerLength]byte

	if _, err := io.ReadAtLeast(r, hdr[:], int(headerLength)); err != nil {
		return noTag, nil, ConnError{err}
	}

	// Decode the header.
	headerBuf := buffer{data: hdr[:]}
	size := headerBuf.Read32()
	t := headerBuf.ReadMsgType()
	tag := headerBuf.ReadTag()
	if size < headerLength {
		// The message is too small.
		//
		// See above: it's probably screwed.
		return noTag, nil, ConnError{ErrNoValidMessage}
	}
	if size > maximumLength || size > msize {
		// The message is too big.
		return noTag, nil, ConnError{&ErrMessageTooLarge{size, msize}}
	}
	remaining := size - headerLength

	// Find our message to decode.
	m, err := lookup(tag, t)
	if err != nil {
		// Throw away the contents of this message.
		if remaining > 0 {
			_, _ = io.Copy(ioutil.Discard, io.LimitReader(r, int64(remaining)))
		}
		return tag, nil, err
	}

	// Not yet initialized.
	var dataBuf buffer
	var vecs vecnet.Buffers

	appendBuffer := func(size int) *[]byte {
		// Pull a data buffer from the pool.
		datap := dataPool.Get().(*[]byte)
		data := *datap
		if size > len(data) {
			// Create a larger data buffer.
			data = make([]byte, size)
			datap = &data
		} else {
			// Limit the data buffer.
			data = data[:size]
		}
		dataBuf = buffer{data: data}
		vecs = append(vecs, data)
		return datap
	}

	// Read the rest of the payload.
	//
	// This requires some special care to ensure that the vectors all line
	// up the way they should. We do this to minimize copying data around.
	if payloader, ok := m.(payloader); ok {
		fixedSize := payloader.FixedSize()

		// Do we need more than there is?
		if fixedSize > remaining {
			// This is not a valid message.
			if remaining > 0 {
				_, _ = io.Copy(ioutil.Discard, io.LimitReader(r, int64(remaining)))
			}
			return noTag, nil, ErrNoValidMessage
		}

		if fixedSize != 0 {
			datap := appendBuffer(int(fixedSize))
			defer dataPool.Put(datap)
		}

		// Include the payload.
		p := payloader.Payload()
		if p == nil || len(p) != int(remaining-fixedSize) {
			p = make([]byte, remaining-fixedSize)
			payloader.SetPayload(p)
		}
		if len(p) > 0 {
			vecs = append(vecs, p)
		}
	} else if remaining != 0 {
		datap := appendBuffer(int(remaining))
		defer dataPool.Put(datap)
	}

	if len(vecs) > 0 {
		if _, err := vecs.ReadFrom(r); err != nil {
			return noTag, nil, ConnError{err}
		}
	}

	// Decode the message data.
	m.decode(&dataBuf)
	if dataBuf.isOverrun() {
		// No need to drain the socket.
		return noTag, nil, ErrNoValidMessage
	}

	l.Printf("recv [r %p] [Tag %06d] %s", r, tag, m)

	// All set.
	return tag, m, nil
}
