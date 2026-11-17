//go:build !unix

package ctxio

import (
	"context"
	"errors"
	"os"
)

// ErrFDsUnsupported is returned when fd passing is attempted over a non-unix transport.
var ErrFDsUnsupported = errors.New("ctxio: fd passing requires a unix socket transport")

// MaxFDs is the maximum number of file descriptors that may be passed in a single message.
// Linux's SCM_MAX_FD is 253; exceeding it causes sendmsg to fail with EINVAL.
const MaxFDs = 253

// CanPassFDs always returns false on non-unix platforms.
func (c *Conn) CanPassFDs() bool {
	return false
}

// drainFDBatches is a no-op on non-unix platforms (there are never any batches).
func (c *Conn) drainFDBatches() {}

// WriteWithFDs returns ErrFDsUnsupported when fds is non-empty.
// With zero fds it delegates to Write.
func (c *Conn) WriteWithFDs(ctx context.Context, buf []byte, fds []*os.File) (int, error) {
	if len(fds) > 0 {
		return 0, ErrFDsUnsupported
	}
	return c.Write(ctx, buf)
}

// ReadBytesWithFDs delegates to ReadBytes and returns nil fds.
func (c *Conn) ReadBytesWithFDs(ctx context.Context, delim byte) ([]byte, []*os.File, error) {
	b, err := c.ReadBytes(ctx, delim)
	return b, nil, err
}
