package mpb

import (
	"io"
	"time"
)

type writeCloser struct {
	io.Writer
}

func (w writeCloser) Close() error {
	if closer, ok := w.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type proxyWriter struct {
	writeCloser
	bar *Bar
}

func (x proxyWriter) Write(p []byte) (int, error) {
	n, err := x.writeCloser.Write(p)
	x.bar.IncrBy(n)
	return n, err
}

type proxyWriteReaderFrom struct {
	proxyWriter
	dst io.ReaderFrom
}

func (x proxyWriteReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return x.dst.ReadFrom(proxyReader{readCloser{r}, x.bar})
}

// ewmaProxyWriteReaderFrom implements its own io.ReaderFrom which will shadow any
// io.ReaderFrom implementation of the underlying writeCloser's io.Writer. This is
// necessary to correctly track ewma counters.
type ewmaProxyWriteReaderFrom struct {
	writeCloser
	bar *Bar
}

// If io.Copy(ewmaProxyWriteReaderFrom, src) is used then this Write method will
// not be used at all. Just keeping it for manual Write cases.
func (x ewmaProxyWriteReaderFrom) Write(p []byte) (int, error) {
	start := time.Now()
	n, err := x.writeCloser.Write(p)
	x.bar.EwmaIncrBy(n, time.Since(start))
	return n, err
}

func (x ewmaProxyWriteReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return copyBuffer(x.bar, x.writeCloser, r, nil)
}

func newProxyWriter(b *Bar, w io.Writer) io.WriteCloser {
	if len(b.ewmaDecorators) != 0 {
		return ewmaProxyWriteReaderFrom{writeCloser{w}, b}
	}
	pw := proxyWriter{writeCloser{w}, b}
	if dst, ok := w.(io.ReaderFrom); ok {
		return proxyWriteReaderFrom{pw, dst}
	}
	return pw
}
