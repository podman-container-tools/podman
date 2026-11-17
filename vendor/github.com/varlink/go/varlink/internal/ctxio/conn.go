package ctxio

import (
	"bytes"
	"context"
	"net"
	"os"
	"time"
)

// readChunk is the size of each underlying socket read performed by ReadBytes
// and ReadBytesWithFDs.
const readChunk = 8192

// fdBatch associates a set of received *os.File values with an offset in readBuf
// of the recvmsg that delivered them. Used by ReadBytesWithFDs on unix.
//
// startOffset is the position of the LAST byte appended to readBuf by the recvmsg
// that carried these fds; see ReadBytesWithFDs for why the last byte (not the
// first) is the reliable anchor. A message occupying readBuf[0:msgEnd] owns all
// batches whose startOffset < msgEnd (their anchor byte fell within that message).
type fdBatch struct {
	startOffset int // position in readBuf of the last byte from this recvmsg
	files       []*os.File
}

// Conn wraps net.Conn with context aware functionality.
type Conn struct {
	conn     net.Conn
	unixConn *net.UnixConn // non-nil when conn is a *net.UnixConn; enables fd passing

	// readBuf holds bytes that have been read from the socket but not yet
	// consumed by a ReadBytes/ReadBytesWithFDs call. It replaces a bufio.Reader:
	// the fd-passing path cannot use bufio because SCM_RIGHTS ancillary data is
	// bound to the exact recvmsg(2) that delivers the bytes, so both read paths
	// share this single leftover buffer.
	readBuf []byte

	// fdBatches is a FIFO of fd batches keyed by their startOffset into readBuf,
	// maintained by ReadBytesWithFDs on unix transports.
	fdBatches []fdBatch
}

// NewConn creates a new context aware Conn.
// If c is a *net.UnixConn, fd passing is enabled (CanPassFDs returns true).
func NewConn(c net.Conn) *Conn {
	uc, _ := c.(*net.UnixConn)
	return &Conn{
		conn:     c,
		unixConn: uc,
	}
}

// aLongTimeAgo is a time in the past that indicates a connection should
// immediately time out.
var aLongTimeAgo = time.Unix(1, 0)

func (c *Conn) NetConn() net.Conn {
	return c.conn
}

// Close releases the Conn's resources and drains any unconsumed fd batches
// to prevent file-descriptor leaks.
func (c *Conn) Close() error {
	c.drainFDBatches()
	return c.conn.Close()
}

// Write writes to the underlying connection.
// It is not safe for concurrent use with itself.
func (c *Conn) Write(ctx context.Context, buf []byte) (int, error) {
	done := make(chan struct{})
	ioInterrupted := context.AfterFunc(ctx, func() {
		c.conn.SetWriteDeadline(aLongTimeAgo)
		close(done)
	})
	n, err := c.conn.Write(buf)
	if !ioInterrupted() {
		<-done
		c.conn.SetWriteDeadline(time.Time{})
		return n, ctx.Err()
	}
	return n, err
}

// Read reads from the underlying connection.
// It is not safe for concurrent use with itself or ReadBytes.
func (c *Conn) Read(ctx context.Context, buf []byte) (int, error) {
	done := make(chan struct{})
	ioInterrupted := context.AfterFunc(ctx, func() {
		c.conn.SetReadDeadline(aLongTimeAgo)
		close(done)
	})
	n, err := c.conn.Read(buf)
	if !ioInterrupted() {
		<-done
		c.conn.SetReadDeadline(time.Time{})
		return n, ctx.Err()
	}
	return n, err
}

// ReadBytes reads from the connection until delim is found, returning the
// accumulated bytes up to and including delim. Bytes read past delim are
// retained for the next call.
//
// It mirrors bufio.Reader.ReadBytes: if an error (including io.EOF) occurs
// before delim is found, it returns the data read so far together with the
// error.
//
// It is not safe for concurrent use with itself or Read.
func (c *Conn) ReadBytes(ctx context.Context, delim byte) ([]byte, error) {
	// scanned tracks how many bytes of readBuf have already been searched for
	// delim, so each byte is examined at most once across loop iterations.
	scanned := 0
	for {
		if idx := bytes.IndexByte(c.readBuf[scanned:], delim); idx >= 0 {
			return c.popMessage(scanned + idx + 1), nil
		}
		scanned = len(c.readBuf)

		// Read the next chunk directly into spare capacity at the tail of
		// readBuf, avoiding a copy through an intermediate buffer.
		c.readBuf = growForRead(c.readBuf, readChunk)
		dst := c.readBuf[len(c.readBuf) : len(c.readBuf)+readChunk]

		// Reuse the existing per-call cancellation dance in Read; it resets the
		// read deadline on every return, so no stale deadline survives across
		// chunks.
		n, err := c.Read(ctx, dst)
		c.readBuf = c.readBuf[:len(c.readBuf)+n]
		if err != nil {
			// The bytes just read may have completed a message; prefer
			// returning it and surface the error on a subsequent call.
			if idx := bytes.IndexByte(c.readBuf[scanned:], delim); idx >= 0 {
				return c.popMessage(scanned + idx + 1), nil
			}
			out := c.readBuf
			c.readBuf = nil
			return out, err
		}
	}
}

// growForRead ensures buf has at least n bytes of spare capacity past its
// length, returning a slice with the same length but room to read into the
// tail. It does not change buf's length.
//
// This is the read-into-spare-capacity pattern used by bytes.Buffer.ReadFrom:
// the caller reads the socket directly into buf[len:len+n] and then extends the
// length. When reallocation is needed it grows geometrically (at least
// doubling, and always enough for n) to amortise allocations over a long
// stream. popMessage reclaims consumed front space on every pop, so in steady
// state this rarely reallocates.
func growForRead(buf []byte, n int) []byte {
	if cap(buf)-len(buf) >= n {
		return buf
	}
	newCap := 2 * cap(buf)
	if newCap < len(buf)+n {
		newCap = len(buf) + n
	}
	grown := make([]byte, len(buf), newCap)
	copy(grown, buf)
	return grown
}

// popMessage detaches and returns readBuf[:n] as an independent slice, keeping
// any remaining bytes for the next read.
//
// The leftover tail (readBuf[n:]) is slid down to the front of the backing
// array rather than re-sliced forward. Re-slicing (readBuf = readBuf[n:]) would
// advance the slice start, permanently stranding the consumed front bytes and
// shrinking cap(readBuf) by n on every pop, forcing growForRead to reallocate
// repeatedly over a long stream. Sliding reclaims that space — the same
// compaction bufio.Reader performs internally.
//
// The slide is transparent to the fd-passing path (ReadBytesWithFDs): its
// fdBatch startOffsets are relative to readBuf[0], and a slide preserves every
// slice-relative index (readBuf[i] is the same logical byte before and after),
// changing only the underlying array index. popMessage is never called from the
// fd path, which pops inline.
func (c *Conn) popMessage(n int) []byte {
	msg := make([]byte, n)
	copy(msg, c.readBuf[:n])
	rest := copy(c.readBuf[:cap(c.readBuf)], c.readBuf[n:])
	c.readBuf = c.readBuf[:rest]
	return msg
}
