package ctxreader

import (
	"context"
	"io"
)

type cancelableReader struct {
	ctx context.Context
	io.Reader
}

func (c *cancelableReader) Read(p []byte) (n int, err error) {
	// this will only even kind of work because we're using it in situations where we don't
	// expect to block while waiting for more data - we're either reading from a file, or from
	// a pipe where the writer is sending us data as fast as it can manage
	select {
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	default:
	}
	return c.Reader.Read(p)
}

func NewCancelableReader(ctx context.Context, r io.Reader) io.Reader {
	return &cancelableReader{
		ctx:    ctx,
		Reader: r,
	}
}
