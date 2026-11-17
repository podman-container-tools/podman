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
	"sync"

	"github.com/hugelgupf/p9/linux"
	"github.com/u-root/uio/ulog"
)

// ErrOutOfTags indicates no tags are available.
var ErrOutOfTags = errors.New("out of tags -- messages lost?")

// ErrOutOfFIDs indicates no more FIDs are available.
var ErrOutOfFIDs = errors.New("out of FIDs -- messages lost?")

// ErrUnexpectedTag indicates a response with an unexpected tag was received.
var ErrUnexpectedTag = errors.New("unexpected tag in response")

// ErrVersionsExhausted indicates that all versions to negotiate have been exhausted.
var ErrVersionsExhausted = errors.New("exhausted all versions to negotiate")

// ErrBadVersionString indicates that the version string is malformed or unsupported.
var ErrBadVersionString = errors.New("bad version string")

// ErrBadResponse indicates the response didn't match the request.
type ErrBadResponse struct {
	Got  msgType
	Want msgType
}

// Error returns a highly descriptive error.
func (e *ErrBadResponse) Error() string {
	return fmt.Sprintf("unexpected message type: got %v, want %v", e.Got, e.Want)
}

// response is the asynchronous return from recv.
//
// This is used in the pending map below.
type response struct {
	r    message
	done chan error
}

var responsePool = sync.Pool{
	New: func() interface{} {
		return &response{
			done: make(chan error, 1),
		}
	},
}

// Client is at least a 9P2000.L client.
type Client struct {
	// conn is the connected conn.
	conn io.ReadWriteCloser

	// tagPool is the collection of available tags.
	tagPool pool

	// fidPool is the collection of available fids.
	fidPool pool

	// pending is the set of pending messages.
	pending   map[tag]*response
	pendingMu sync.Mutex

	// sendMu is the lock for sending a request.
	sendMu sync.Mutex

	// recvr is essentially a mutex for calling recv.
	//
	// Whoever writes to this channel is permitted to call recv. When
	// finished calling recv, this channel should be emptied.
	recvr chan bool

	// messageSize is the maximum total size of a message.
	messageSize uint32

	// payloadSize is the maximum payload size of a read or write
	// request.  For large reads and writes this means that the
	// read or write is broken up into buffer-size/payloadSize
	// requests.
	payloadSize uint32

	// version is the agreed upon version X of 9P2000.L.Google.X.
	// version 0 implies 9P2000.L.
	version uint32

	// log is the logger to write to, if specified.
	log ulog.Logger
}

// ClientOpt enables optional client configuration.
type ClientOpt func(*Client) error

// WithMessageSize overrides the default message size.
func WithMessageSize(m uint32) ClientOpt {
	return func(c *Client) error {
		// Need at least one byte of payload.
		if m <= msgDotLRegistry.largestFixedSize {
			return &ErrMessageTooLarge{
				size:  m,
				msize: msgDotLRegistry.largestFixedSize,
			}
		}
		c.messageSize = m
		return nil
	}
}

// WithClientLogger overrides the default logger for the client.
func WithClientLogger(l ulog.Logger) ClientOpt {
	return func(c *Client) error {
		c.log = l
		return nil
	}
}

func roundDown(p uint32, align uint32) uint32 {
	if p > align && p%align != 0 {
		return p - p%align
	}
	return p
}

// NewClient creates a new client. It performs a Tversion exchange with
// the server to assert that messageSize is ok to use.
//
// You should not use the same conn for multiple clients.
func NewClient(conn io.ReadWriteCloser, o ...ClientOpt) (*Client, error) {
	c := &Client{
		conn:        conn,
		tagPool:     pool{start: 1, limit: uint64(noTag)},
		fidPool:     pool{start: 1, limit: uint64(noFID)},
		pending:     make(map[tag]*response),
		recvr:       make(chan bool, 1),
		messageSize: DefaultMessageSize,
		log:         ulog.Null,

		// Request a high version by default.
		version: highestSupportedVersion,
	}

	for _, opt := range o {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Compute a payload size and round to 512 (normal block size)
	// if it's larger than a single block.
	c.payloadSize = roundDown(c.messageSize-msgDotLRegistry.largestFixedSize, 512)

	// Agree upon a version.
	requested := c.version
	for {
		rversion := rversion{}
		err := c.sendRecv(&tversion{Version: versionString(version9P2000L, requested), MSize: c.messageSize}, &rversion)

		// The server told us to try again with a lower version.
		if errors.Is(err, linux.EAGAIN) {
			if requested == lowestSupportedVersion {
				return nil, ErrVersionsExhausted
			}
			requested--
			continue
		}

		// We requested an impossible version or our other parameters were bogus.
		if err != nil {
			return nil, err
		}

		// Parse the version.
		baseVersion, version, ok := parseVersion(rversion.Version)
		if !ok {
			// The server gave us a bad version. We return a generically worrisome error.
			c.log.Printf("server returned bad version string %q", rversion.Version)
			return nil, ErrBadVersionString
		}
		if baseVersion != version9P2000L {
			c.log.Printf("server returned unsupported base version %q (version %q)", baseVersion, rversion.Version)
			return nil, ErrBadVersionString
		}
		c.version = version
		break
	}
	return c, nil
}

// handleOne handles a single incoming message.
//
// This should only be called with the token from recvr. Note that the received
// tag will automatically be cleared from pending.
func (c *Client) handleOne() {
	t, r, err := recv(c.log, c.conn, c.messageSize, func(t tag, mt msgType) (message, error) {
		c.pendingMu.Lock()
		resp := c.pending[t]
		c.pendingMu.Unlock()

		// Not expecting this message?
		if resp == nil {
			c.log.Printf("client received unexpected tag %v, ignoring", t)
			return nil, ErrUnexpectedTag
		}

		// Is it an error? We specifically allow this to
		// go through, and then we deserialize below.
		if mt == msgRlerror {
			return &rlerror{}, nil
		}

		// Does it match expectations?
		if mt != resp.r.typ() {
			return nil, &ErrBadResponse{Got: mt, Want: resp.r.typ()}
		}

		// Return the response.
		return resp.r, nil
	})

	if err != nil {
		// No tag was extracted (probably a conn error).
		//
		// Likely catastrophic. Notify all waiters and clear pending.
		c.pendingMu.Lock()
		for _, resp := range c.pending {
			resp.done <- err
		}
		c.pending = make(map[tag]*response)
		c.pendingMu.Unlock()
	} else {
		// Process the tag.
		//
		// We know that is is contained in the map because our lookup function
		// above must have succeeded (found the tag) to return nil err.
		c.pendingMu.Lock()
		resp := c.pending[t]
		delete(c.pending, t)
		c.pendingMu.Unlock()
		resp.r = r
		resp.done <- err
	}
}

// waitAndRecv co-ordinates with other receivers to handle responses.
func (c *Client) waitAndRecv(done chan error) error {
	for {
		select {
		case err := <-done:
			return err
		case c.recvr <- true:
			select {
			case err := <-done:
				// It's possible that we got the token, despite
				// done also being available. Check for that.
				<-c.recvr
				return err
			default:
				// Handle receiving one tag.
				c.handleOne()

				// Return the token.
				<-c.recvr
			}
		}
	}
}

// sendRecv performs a roundtrip message exchange.
//
// This is called by internal functions.
func (c *Client) sendRecv(tm message, rm message) error {
	t, ok := c.tagPool.Get()
	if !ok {
		return ErrOutOfTags
	}
	defer c.tagPool.Put(t)

	// Indicate we're expecting a response.
	//
	// Note that the tag will be cleared from pending
	// automatically (see handleOne for details).
	resp := responsePool.Get().(*response)
	defer responsePool.Put(resp)
	resp.r = rm
	c.pendingMu.Lock()
	c.pending[tag(t)] = resp
	c.pendingMu.Unlock()

	// Send the request over the wire.
	c.sendMu.Lock()
	err := send(c.log, c.conn, tag(t), tm)
	c.sendMu.Unlock()
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}

	// Co-ordinate with other receivers.
	if err := c.waitAndRecv(resp.done); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	// Is it an error message?
	//
	// For convenience, we transform these directly
	// into errors. Handlers need not handle this case.
	if rlerr, ok := resp.r.(*rlerror); ok {
		return linux.Errno(rlerr.Error)
	}

	// At this point, we know it matches.
	//
	// Per recv call above, we will only allow a type
	// match (and give our r) or an instance of Rlerror.
	return nil
}

// Version returns the negotiated 9P2000.L.Google version number.
func (c *Client) Version() uint32 {
	return c.version
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
