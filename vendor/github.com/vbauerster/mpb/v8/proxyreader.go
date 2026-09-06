package mpb

import (
	"io"
	"time"
)

type readCloser struct {
	io.Reader
}

func (r readCloser) Close() error {
	if closer, ok := r.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type proxyReader struct {
	readCloser
	bar *Bar
}

func (x proxyReader) Read(p []byte) (int, error) {
	n, err := x.readCloser.Read(p)
	x.bar.IncrBy(n)
	return n, err
}

type proxyReadWriterTo struct {
	proxyReader
	src io.WriterTo
}

func (x proxyReadWriterTo) WriteTo(w io.Writer) (int64, error) {
	return x.src.WriteTo(proxyWriter{writeCloser{w}, x.bar})
}

// ewmaProxyReadWriterTo implements its own io.WriterTo which will shadow any
// io.WriterTo implementation of the underlying readCloser's io.Reader. This is
// necessary to correctly track ewma counters.
type ewmaProxyReadWriterTo struct {
	readCloser
	bar *Bar
}

// If io.Copy(dst, ewmaProxyReadWriterTo) is used then this Read method will
// not be used at all. Just keeping it for manual Read cases.
func (x ewmaProxyReadWriterTo) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := x.readCloser.Read(p)
	x.bar.EwmaIncrBy(n, time.Since(start))
	return n, err
}

//nolint:staticcheck // QF1008
func (x ewmaProxyReadWriterTo) WriteTo(w io.Writer) (int64, error) {
	return copyBuffer(x.bar, w, x.readCloser.Reader, nil)
}

func newProxyReader(b *Bar, r io.Reader) io.ReadCloser {
	if len(b.ewmaDecorators) != 0 {
		return ewmaProxyReadWriterTo{readCloser{r}, b}
	}
	pr := proxyReader{readCloser{r}, b}
	if src, ok := r.(io.WriterTo); ok {
		return proxyReadWriterTo{pr, src}
	}
	return pr
}
