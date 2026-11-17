//go:build unix

package ctxio

import (
	"bytes"
	"context"
	"errors"
	"os"
	"runtime"
	"syscall"
	"time"
)

// ErrFDsUnsupported is returned when fd passing is attempted over a non-unix transport.
var ErrFDsUnsupported = errors.New("ctxio: fd passing requires a unix socket transport")

// MaxFDs is the maximum number of file descriptors that may be passed in a single message.
// Linux's SCM_MAX_FD is 253; exceeding it causes sendmsg to fail with EINVAL.
const MaxFDs = 253

// CanPassFDs reports whether the connection supports fd passing.
// On Unix it is true iff the underlying connection is a *net.UnixConn.
func (c *Conn) CanPassFDs() bool {
	return c.unixConn != nil
}

// drainFDBatches closes all files remaining in queued fd batches and clears the
// queue. Called both from Close() and from ReadBytesWithFDs whenever it hits an
// unrecoverable error, so fds queued for a message that will never complete
// don't leak until some later (or absent) Close() call.
func (c *Conn) drainFDBatches() {
	for _, batch := range c.fdBatches {
		for _, f := range batch.files {
			if f != nil {
				f.Close()
			}
		}
	}
	c.fdBatches = nil
}

// WriteWithFDs writes buf to the connection, attaching fds as SCM_RIGHTS ancillary data
// on the first (and only) sendmsg call. If the write is short, the remainder is sent
// without ancillary data (fds are already delivered with the first segment).
//
// Ownership: the caller retains ownership of fds; the library never closes them.
// runtime.KeepAlive is called after the syscall to prevent the GC from finalising them
// mid-syscall.
func (c *Conn) WriteWithFDs(ctx context.Context, buf []byte, fds []*os.File) (int, error) {
	if c.unixConn == nil {
		if len(fds) > 0 {
			return 0, ErrFDsUnsupported
		}
		return c.Write(ctx, buf)
	}

	// With no fds, skip WriteMsgUnix entirely: Write uses the same underlying
	// connection and avoids duplicating the context-cancel deadline dance.
	if len(fds) == 0 {
		return c.Write(ctx, buf)
	}

	intfds := make([]int, len(fds))
	for i, f := range fds {
		intfds[i] = int(f.Fd())
	}
	oob := syscall.UnixRights(intfds...)

	done := make(chan struct{})
	ioInterrupted := context.AfterFunc(ctx, func() {
		c.unixConn.SetWriteDeadline(aLongTimeAgo)
		close(done)
	})

	n, _, err := c.unixConn.WriteMsgUnix(buf, oob, nil)

	// REQUIRED: prevent GC from closing fds before the syscall completes.
	runtime.KeepAlive(fds)

	if !ioInterrupted() {
		<-done
		c.unixConn.SetWriteDeadline(time.Time{})
		return n, ctx.Err()
	}

	if err != nil {
		return n, err
	}

	// If WriteMsgUnix wrote fewer bytes than buf, send the remainder without oob.
	// FDs are already delivered with the first segment.
	if n < len(buf) {
		nn, werr := c.Write(ctx, buf[n:])
		return n + nn, werr
	}

	return n, nil
}

// ReadBytesWithFDs reads bytes until delim, returning the accumulated bytes and any
// file descriptors received via SCM_RIGHTS ancillary data on the recvmsg(s) that
// delivered the bytes of this message.
//
// On a unix connection each fill is a recvmsg(2): ancillary data is bound to the
// exact recvmsg that delivered the payload bytes, which is why the read path is
// buffer-based rather than using bufio (read-ahead would silently discard oob
// data). It shares Conn.readBuf with ReadBytes, so the two read paths can be
// interleaved on one connection without losing buffered bytes.
//
// FD→message association: fds that arrive on a recvmsg belong to the message that
// contains the LAST BYTE of that recvmsg's payload. The sender emits each message
// as a single sendmsg with the fds attached (matching the zlink wire spec), and
// the kernel ends its stream-receive loop as soon as it attaches an skb's fds, so
// the fd-bearing recvmsg's bytes end inside that message. Each fdBatch records the
// startOffset = position in readBuf of that last byte. When popping a message at
// [0, idx+1), all batches whose startOffset < idx+1 belong to that message.
//
// Keying on the first byte instead would be unsound: the kernel can coalesce a
// preceding fd-less message in front of the fd-bearing one in a single recvmsg,
// placing the first byte in the earlier message and misattributing the fds to it.
//
// Ownership: returned *os.File values are fresh dups owned by the caller, who must
// Close them. If the service handler does not claim them, the service closes them.
func (c *Conn) ReadBytesWithFDs(ctx context.Context, delim byte) ([]byte, []*os.File, error) {
	if c.unixConn == nil {
		b, err := c.ReadBytes(ctx, delim)
		return b, nil, err
	}

	for {
		// Check if readBuf already contains delim.
		if idx := bytes.IndexByte(c.readBuf, delim); idx >= 0 {
			msg := make([]byte, idx+1)
			copy(msg, c.readBuf[:idx+1])
			c.readBuf = c.readBuf[idx+1:]

			// Pop all batches whose startOffset < idx+1: their first bytes fell
			// within this message, so this message owns their fds.
			files := c.popFDBatchesForMessage(idx + 1)
			// Adjust startOffsets of remaining batches.
			c.adjustFDBatchOffsets(idx + 1)

			return msg, files, nil
		}

		// Need more data: do one recvmsg.
		p := make([]byte, readChunk)
		oob := make([]byte, syscall.CmsgSpace(MaxFDs*4))

		done := make(chan struct{})
		ioInterrupted := context.AfterFunc(ctx, func() {
			c.unixConn.SetReadDeadline(aLongTimeAgo)
			close(done)
		})

		n, oobn, flags, _, err := c.unixConn.ReadMsgUnix(p, oob)

		if !ioInterrupted() {
			<-done
			c.unixConn.SetReadDeadline(time.Time{})
			if n == 0 {
				// No message will ever complete now: close fds queued for
				// the in-flight message rather than stranding them until
				// Close() (direct callers of ReadBytesWithFDs may not call
				// Close immediately, or at all, after an error).
				c.drainFDBatches()
				return nil, nil, ctx.Err()
			}
			// n > 0: we got data; surface the ctx error after draining.
			err = ctx.Err()
		}

		if flags&syscall.MSG_CTRUNC != 0 {
			// The kernel may have already installed some fds before truncating.
			// Parse and close them to avoid leaks, then return the error.
			if oobn > 0 {
				if partialFiles, ferr := parseSCMRights(oob[:oobn]); ferr == nil {
					for _, f := range partialFiles {
						if f != nil {
							f.Close()
						}
					}
				}
			}
			// This message is unrecoverable, and so is the connection (the
			// stream is desynchronized past this point). Also close fds
			// queued by any earlier, still-incomplete batch rather than
			// stranding them until Close().
			c.drainFDBatches()
			return nil, nil, errors.New("ctxio: ancillary data truncated (MSG_CTRUNC); too many fds")
		}

		// Parse any SCM_RIGHTS messages and enqueue them, keyed to the LAST byte
		// of this recvmsg's payload. The kernel ends its stream-receive loop as
		// soon as it attaches an skb's fds, so an fd-bearing recvmsg's bytes end
		// at this last byte; per the wire spec each message is a single sendmsg,
		// so the fds belong to the message containing that byte. Keying on the
		// first byte would be unsound: the kernel can coalesce a preceding
		// fd-less message in front of the fd-bearing one, putting the recvmsg's
		// first byte in the earlier message and stealing its fds.
		if oobn > 0 && n > 0 {
			files, ferr := parseSCMRights(oob[:oobn])
			if ferr != nil {
				// Malformed ancillary data leaves the connection unusable;
				// close fds from any earlier, still-incomplete batch rather
				// than stranding them until Close().
				c.drainFDBatches()
				return nil, nil, ferr
			}
			if len(files) > 0 {
				c.fdBatches = append(c.fdBatches, fdBatch{
					startOffset: len(c.readBuf) + n - 1, // position of last byte from this recvmsg
					files:       files,
				})
			}
		}

		if n > 0 {
			c.readBuf = append(c.readBuf, p[:n]...)
		}

		if err != nil {
			if n == 0 {
				// The peer is gone and no more bytes are coming: any fds
				// already queued for the still-incomplete message would
				// otherwise leak until Close() is eventually called.
				c.drainFDBatches()
				return nil, nil, err
			}
			// There is data; continue looping to drain the buffer.
			// If the buffer already contains delim the next iteration returns it.
		}
	}
}

// popFDBatchesForMessage pops and returns all fds from batches whose startOffset is
// less than msgEnd (i.e. their first byte fell within the message occupying [0, msgEnd)).
// Batches are consumed in FIFO order and their files concatenated.
func (c *Conn) popFDBatchesForMessage(msgEnd int) []*os.File {
	var files []*os.File
	for len(c.fdBatches) > 0 && c.fdBatches[0].startOffset < msgEnd {
		files = append(files, c.fdBatches[0].files...)
		c.fdBatches = c.fdBatches[1:]
	}
	return files
}

// adjustFDBatchOffsets subtracts consumed bytes from all remaining batch startOffsets.
func (c *Conn) adjustFDBatchOffsets(consumed int) {
	for i := range c.fdBatches {
		c.fdBatches[i].startOffset -= consumed
	}
}

// parseSCMRights parses ancillary socket control messages and returns any file
// descriptors found in SCM_RIGHTS messages. Other cmsg types (e.g. SCM_CREDENTIALS)
// are silently ignored, matching the zlink/Rust wire spec.
//
// On error, all *os.File values already created are closed before returning to
// prevent descriptor leaks.
func parseSCMRights(oob []byte) ([]*os.File, error) {
	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, err
	}

	var files []*os.File
	for _, msg := range msgs {
		if msg.Header.Level != syscall.SOL_SOCKET || msg.Header.Type != syscall.SCM_RIGHTS {
			continue
		}
		rawFDs, err := syscall.ParseUnixRights(&msg)
		if err != nil {
			// Close already-created files before returning to avoid leaks.
			for _, f := range files {
				f.Close()
			}
			return nil, err
		}
		for _, fd := range rawFDs {
			files = append(files, os.NewFile(uintptr(fd), "varlink-fd"))
		}
	}
	return files, nil
}
