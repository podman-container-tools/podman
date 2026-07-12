package mpb

import (
	"io"
	"time"
)

type proxyWriteCloser struct {
	w   io.Writer
	bar *Bar
}

func (x proxyWriteCloser) Write(p []byte) (int, error) {
	n, err := x.w.Write(p)
	x.bar.IncrBy(n)
	return n, err
}

func (x proxyWriteCloser) Close() error {
	if wc, ok := x.w.(io.WriteCloser); ok {
		return wc.Close()
	}
	return nil
}

type proxyReaderFrom struct {
	proxyWriteCloser
	rf io.ReaderFrom
}

func (x proxyReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return x.rf.ReadFrom(proxyReadCloser{r, x.bar})
}

type ewmaProxyWriteCloser struct {
	w   io.Writer
	bar *Bar
}

func (x ewmaProxyWriteCloser) Write(p []byte) (int, error) {
	start := time.Now()
	n, err := x.w.Write(p)
	x.bar.EwmaIncrBy(n, time.Since(start))
	return n, err
}

func (x ewmaProxyWriteCloser) Close() error {
	if wc, ok := x.w.(io.WriteCloser); ok {
		return wc.Close()
	}
	return nil
}

type ewmaProxyReaderFrom struct {
	ewmaProxyWriteCloser
	rf io.ReaderFrom
}

func (x ewmaProxyReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return x.rf.ReadFrom(ewmaProxyReadCloser{r, x.bar})
}

func newProxyWriter(w io.Writer, b *Bar, hasEwma bool) io.WriteCloser {
	if hasEwma {
		epw := ewmaProxyWriteCloser{w, b}
		if rf, ok := w.(io.ReaderFrom); ok {
			return ewmaProxyReaderFrom{epw, rf}
		}
		return epw
	}
	pw := proxyWriteCloser{w, b}
	if rf, ok := w.(io.ReaderFrom); ok {
		return proxyReaderFrom{pw, rf}
	}
	return pw
}
