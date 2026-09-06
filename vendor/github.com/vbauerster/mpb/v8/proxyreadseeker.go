package mpb

import (
	"io"
	"time"
)

type proxyReadSeeker struct {
	rs  io.ReadSeeker
	bar *Bar
}

func (x proxyReadSeeker) Read(p []byte) (int, error) {
	n, err := x.rs.Read(p)
	x.bar.IncrBy(n)
	return n, err
}

func (x proxyReadSeeker) Seek(offset int64, whence int) (int64, error) {
	n, err := x.rs.Seek(offset, whence)
	if err == nil {
		x.bar.SetCurrent(n)
	}
	return n, err
}

func (x proxyReadSeeker) Close() error {
	if closer, ok := x.rs.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type ewmaProxyReadSeeker struct {
	rs  io.ReadSeeker
	bar *Bar
}

func (x ewmaProxyReadSeeker) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := x.rs.Read(p)
	x.bar.EwmaIncrBy(n, time.Since(start))
	return n, err
}

func (x ewmaProxyReadSeeker) Seek(offset int64, whence int) (int64, error) {
	n, err := x.rs.Seek(offset, whence)
	if err == nil {
		x.bar.EwmaSetCurrent(n, 0)
	}
	return n, err
}

func (x ewmaProxyReadSeeker) Close() error {
	if closer, ok := x.rs.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func newProxyReadSeeker(b *Bar, rs io.ReadSeeker) io.ReadSeekCloser {
	if len(b.ewmaDecorators) != 0 {
		return ewmaProxyReadSeeker{rs, b}
	}
	return proxyReadSeeker{rs, b}
}
